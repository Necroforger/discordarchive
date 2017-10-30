package discordarchive

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
)

// errors
var (
	ErrNoChannels = errors.New("error, no channels found in guild")
	ErrNotUnique  = errors.New("error, value does not meet the unique constraint")
)

var numdownloadtokens = 3

// Options stores optional parameters
type Options struct {
	// SaveAttachments enables saving attachment images to disk
	// applies to: ArchiveGuild, ArchiveChannel.
	SaveAttachments bool // default: false

	// SaveEmbedImages enables saving images from embeds to disk
	// applies to: ArchiveGuild, ArchiveChannel.
	SaveEmbedImages bool // default: false

	// SaveAvatars enables saving user avatars to disk
	// applies to: ArchiveMembers.
	SaveAvatars bool // default: false

	// AvatarSize is the size of the avatar image as a power of two
	// That will be downloaded.
	AvatarSize string // default: ""

	// Limit is the maximum number of messages to archive per channel
	// If the value is 0 or less, it will be ignored.
	// Applies to: ArchiveGuild, ArchiveChannel.
	// When used in ArchiveGuild, it will apply the limit to every channel.
	Limit int // default: 0

	// Skips n items before beginning archive.
	// applies to: ArchiveGuild, ArchiveChannel, ArchiveMembers.
	Skip int // default: 0

	// LastID is the id of the item to retrieve items before or after.
	// Applies to: ArchiveChannel, ArchiveMembers.
	LastID string // default: ""
}

// NewOptions returns a pointer to an options struct initialized with the
// default values.
func NewOptions() *Options {
	opt := &Options{
		SaveAttachments: false,
		SaveEmbedImages: false,
		SaveAvatars:     false,
		AvatarSize:      "",
		Limit:           0,
		Skip:            0,
		LastID:          "",
	}
	return opt
}

// Archiver archives
type Archiver struct {
	// Print various log information
	Log io.Writer

	// SavePath is the folder to save embeds and attachments to.
	// If the folder does not exists, it will be created.
	// Defaults to './'
	SavePath string

	// store a list of unknown members to prevent excess api queries.
	// map(guildID+userid)
	unknownMembers map[string]bool

	// archivedMembers map(guildid+userid) stores whether a particular member has already been archived.
	archivedMembers map[string]bool
	// nicknames maps (guildid+userid) to their nicknames
	nicknames map[string]string
	// limits the amount of actively downloading files
	downloadTokens chan struct{}
	// custom http client for downloading files
	httpclient *http.Client
}

// New returns a new archiver
func New() *Archiver {
	a := &Archiver{
		Log:             nil,
		SavePath:        "./",
		unknownMembers:  map[string]bool{},
		archivedMembers: map[string]bool{},
		nicknames:       map[string]string{},
		downloadTokens:  make(chan struct{}, numdownloadtokens),
		httpclient: &http.Client{
			Timeout: time.Second * 15,
		},
	}
	// Fill the download queue with tokens
	for i := 0; i < numdownloadtokens; i++ {
		a.downloadTokens <- struct{}{}
	}
	return a
}

func (a *Archiver) logf(str string, args ...interface{}) {
	if a.Log != nil {
		fmt.Fprintf(a.Log, str+"\n", args...)
	}
}

func (a *Archiver) log(args ...interface{}) {
	if a.Log != nil {
		fmt.Fprintln(a.Log, args...)
	}
}

func (a *Archiver) isMemberUnknown(guildID, userID string) bool {
	if v, ok := a.unknownMembers[guildID+userID]; ok {
		return v
	}
	return false
}

func (a *Archiver) isMemberArchived(guildID, userID string) bool {
	if v, ok := a.archivedMembers[guildID+userID]; ok {
		return v
	}
	return false
}

// InitDB initializes the database with the required tables
func (a *Archiver) InitDB(tx *sql.Tx, opt *Options) error {
	if opt == nil {
		opt = NewOptions()
	}
	_, err := tx.Exec(
		"CREATE TABLE IF NOT EXISTS messages(" +
			"channelID TEXT, " +
			"messageID TEXT, " +
			"userID TEXT, " +
			"username TEXT, " +
			"nickname TEXT, " +
			"content TEXT, " +
			"embedsJSON TEXT, " +
			"attachmentsJSON TEXT, " +
			"UNIQUE(channelID, messageID))",
	)
	if err != nil {
		return err
	}

	_, err = tx.Exec(
		"CREATE TABLE IF NOT EXISTS channels(" +
			"channelID TEXT NOT NULL UNIQUE, " +
			"guildID TEXT, " +
			"name TEXT, " +
			"topic TEXT, " +
			"type INT, " +
			"channelJSON TEXT)",
	)
	if err != nil {
		return err
	}

	_, err = tx.Exec(
		"CREATE TABLE IF NOT EXISTS guilds(" +
			"guildID TEXT NOT NULL UNIQUE, " +
			"name TEXT, " +
			"guildJSON TEXT)",
	)
	if err != nil {
		return err
	}

	_, err = tx.Exec(
		"CREATE TABLE IF NOT EXISTS files(" +
			"channelID TEXT, " +
			"messageID TEXT, " +
			"path TEXT UNIQUE)",
	)
	if err != nil {
		return err
	}

	_, err = tx.Exec(
		"CREATE TABLE IF NOT EXISTS avatarfiles(" +
			"userID TEXT UNIQUE, " +
			"path TEXT UNIQUE)",
	)
	if err != nil {
		return err
	}

	_, err = tx.Exec(
		"CREATE TABLE IF NOT EXISTS members(" +
			"guildID TEXT, " +
			"userID TEXT, " +
			"username TEXT, " +
			"nickname TEXT, " +
			"rolesJSON TEXT, " +
			"UNIQUE(guildID, userID)" +
			")",
	)
	if err != nil {
		return err
	}

	_, err = tx.Exec(
		"CREATE TABLE IF NOT EXISTS users(" +
			"userID TEXT UNIQUE, " +
			"username TEXT, " +
			"avatar TEXT, " +
			"discriminator TEXT, " +
			"verified INT" +
			")",
	)

	// Create attachments and embeds folder.
	if opt.SaveAttachments ||
		opt.SaveEmbedImages ||
		opt.SaveAvatars {
		err = os.MkdirAll(a.SavePath, 0600)
		if err != nil {
			return err
		}
	}

	return nil
}

// InsertGuild inserts or updates a guild into the database
func (a *Archiver) InsertGuild(tx *sql.Tx, guild *discordgo.Guild) error {
	guildJSON, err := json.Marshal(guild)
	if err != nil {
		return err
	}

	smt, err := tx.Prepare("INSERT or REPLACE INTO guilds VALUES(?, ?, ?)")
	if err != nil {
		return err
	}
	if _, err = smt.Exec(guild.ID, guild.Name, string(guildJSON)); err != nil {
		return err
	}

	return nil
}

// InsertChannel inserts a channel into the database
func (a *Archiver) InsertChannel(tx *sql.Tx, channel *discordgo.Channel) error {
	channelJSON, err := json.Marshal(channel)
	if err != nil {
		return err
	}

	// Insert channel information into database
	smt, err := tx.Prepare("INSERT or REPLACE INTO channels VALUES(?, ?, ?, ?, ?, ?)")
	if err != nil {
		return err
	}

	_, err = smt.Exec(channel.ID, channel.GuildID, channel.Name, channel.Topic, int(channel.Type), string(channelJSON))
	if err != nil {
		return ErrNotUnique
	}
	smt.Close()

	return nil
}

// InsertMessage inserts a message into the database
func (a *Archiver) InsertMessage(s *discordgo.Session, guildID string, tx *sql.Tx, msg *discordgo.Message, opt *Options) error {
	var (
		nickname        string
		attachmentsJSON string
		embedsJSON      string
	)

	if opt == nil {
		opt = NewOptions()
	}

	if v, ok := a.nicknames[guildID+msg.Author.ID]; ok {
		nickname = v
	} else {
		// attempt to obtain a user's nickname
		if !a.isMemberUnknown(guildID, msg.Author.ID) {
			member, err := s.State.Member(guildID, msg.Author.ID)
			if err != nil {
				member, err = s.GuildMember(guildID, msg.Author.ID)
			}
			if member != nil {
				nickname = member.Nick
				a.nicknames[guildID+msg.Author.ID] = member.Nick
			} else {
				a.unknownMembers[guildID+msg.Author.ID] = true
				a.logf("[warning] member [%s] not found. adding to list of unknown members", msg.Author.Username)
			}
		}
	}

	if a, err := json.Marshal(msg.Attachments); err == nil {
		attachmentsJSON = string(a)
	}
	if e, err := json.Marshal(msg.Embeds); err == nil {
		embedsJSON = string(e)
	}

	smt, err := tx.Prepare("INSERT INTO messages VALUES(?, ?, ?, ?, ?, ?, ?, ?)")
	if err != nil {
		return err
	}
	if _, err = smt.Exec(msg.ChannelID, msg.ID, msg.Author.ID, msg.Author.Username, nickname, msg.Content, embedsJSON, attachmentsJSON); err != nil {
		return err
	}
	smt.Close()

	return nil
}

// InsertMember inserts a member into the members table.
func (a *Archiver) InsertMember(tx *sql.Tx, m *discordgo.Member) error {

	var rolesJSON string
	if j, err := json.Marshal(m.Roles); err == nil {
		rolesJSON = string(j)
	}

	smt, err := tx.Prepare("INSERT OR REPLACE INTO members VALUES(?, ?, ?, ?, ?)")
	if err != nil {
		return err
	}
	defer smt.Close()
	_, err = smt.Exec(m.GuildID, m.User.ID, m.User.Username, m.Nick, rolesJSON)

	return err
}

// InsertUser inserts a user into the users table.
func (a *Archiver) InsertUser(tx *sql.Tx, usr *discordgo.User) error {
	smt, err := tx.Prepare("INSERT OR REPLACE INTO users VALUES(?, ?, ?, ?, ?)")
	if err != nil {
		return err
	}
	defer smt.Close()

	var verified int
	if usr.Verified {
		verified = 1
	} else {
		verified = 0
	}

	_, err = smt.Exec(usr.ID, usr.Username, usr.AvatarURL(""), usr.Discriminator, verified)

	return err
}

// InsertFile inserts a file into the database
func (a *Archiver) InsertFile(tx *sql.Tx, channelID, messageID, path string) error {
	smt, err := tx.Prepare("INSERT INTO files VALUES(?, ?, ?)")
	if err != nil {
		return err
	}
	defer smt.Close()
	_, err = smt.Exec(messageID, channelID, path)
	if err != nil {
		return errors.New("[error] error inserting file " + path + " " + err.Error())
	}
	return nil
}

func (a *Archiver) downloadAttachments(tx *sql.Tx, msg *discordgo.Message) error {
	if len(msg.Attachments) != 0 {
		os.MkdirAll(filepath.Join(a.SavePath, "attachments", msg.ChannelID), 0600)
	}

	for i, v := range msg.Attachments {
		pathA := filepath.Join("attachments", msg.ChannelID, fmt.Sprintf("%s-%d-%s", msg.ID, i, v.Filename))
		path := filepath.Join(a.SavePath, pathA)

		resp, err := http.Get(v.URL)
		if err != nil {
			a.logf("[error] error downloading attachment [%s] for message [%s]", v.Filename, msg.ID)
			return nil
		}
		defer resp.Body.Close()

		err = a.InsertFile(tx, msg.ChannelID, msg.ID, pathA)
		if err != nil {
			return err
		}

		f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY, 0600)
		if err != nil {
			return err
		}
		defer f.Close()
		io.Copy(f, resp.Body)
	}

	return nil
}

func (a *Archiver) downloadEmbeds(tx *sql.Tx, msg *discordgo.Message) error {
	if len(msg.Embeds) != 0 {
		os.MkdirAll(filepath.Join(a.SavePath, "embeds", msg.ChannelID), 0600)
	}

	for i, v := range msg.Embeds {
		if v.Image != nil && v.Image.URL != "" {

			resp, err := a.httpclient.Get(v.Image.URL)
			if err != nil {
				return err
			}
			defer resp.Body.Close()

			// Infer the file type
			sample := make([]byte, 512)
			nread, err := resp.Body.Read(sample)
			if err != nil {
				return err
			}
			extension := strings.Split(http.DetectContentType(sample[:nread]), "/")[1]

			if len(extension) <= 4 {

				pathA := filepath.Join("embeds", msg.ChannelID, fmt.Sprintf("%s-%d.%s", msg.ID, i, extension))
				path := filepath.Join(a.SavePath, pathA)

				err = a.InsertFile(tx, msg.ChannelID, msg.ID, pathA)
				if err != nil {
					return err
				}

				f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY, 0600)
				if err != nil {
					return err
				}
				defer f.Close()

				// Write the sample bytes to the file
				_, err = f.Write(sample[:nread])
				if err != nil {
					return err
				}
				_, err = io.Copy(f, resp.Body)
				if err != nil {
					return err
				}
			}
			resp.Body.Close()
		}

		if v.Thumbnail != nil && v.Thumbnail.URL != "" {
			resp, err := a.httpclient.Get(v.Thumbnail.URL)
			if err != nil {
				return err
			}
			defer resp.Body.Close()

			// Infer the file type
			sample := make([]byte, 512)
			nread, err := resp.Body.Read(sample)
			if err != nil {
				return err
			}
			extension := strings.Split(http.DetectContentType(sample[:nread]), "/")[1]

			if len(extension) <= 4 {
				pathA := filepath.Join("embeds", msg.ChannelID, fmt.Sprintf("%s-%d-thumb.%s", msg.ID, i, extension))
				path := filepath.Join(a.SavePath, pathA)

				err = a.InsertFile(tx, msg.ChannelID, msg.ID, pathA)
				if err != nil {
					return err
				}

				f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY, 0600)
				if err != nil {
					return err
				}
				defer f.Close()

				// Write the sample bytes to the file
				_, err = f.Write(sample[:nread])
				if err != nil {
					return err
				}
				_, err = io.Copy(f, resp.Body)
				if err != nil {
					return err
				}
			}
		}
	}
	return nil
}

// downloadAvatar downloads a user's avatar.
func (a *Archiver) downloadAvatar(tx *sql.Tx, usr *discordgo.User, opt *Options) error {
	if opt == nil {
		opt = NewOptions()
	}

	resp, err := http.Get(usr.AvatarURL(opt.AvatarSize))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	sample := make([]byte, 512)
	nsample, err := resp.Body.Read(sample)
	if err != nil {
		return err
	}
	extension := strings.Split(http.DetectContentType(sample), "/")[1]

	pathA := filepath.Join("avatars", fmt.Sprintf("%s.%s", usr.ID, extension))
	path := filepath.Join(a.SavePath, pathA)

	os.MkdirAll(filepath.Join(a.SavePath, "avatars"), 0600)

	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE, 0600)
	if err != nil {
		return err
	}

	_, err = f.Write(sample[:nsample])
	if err != nil {
		return err
	}

	_, err = io.Copy(f, resp.Body)
	if err != nil {
		return err
	}

	smt, err := tx.Prepare("INSERT OR REPLACE INTO avatarfiles VALUES(?, ?)")
	if err != nil {
		return err
	}
	defer smt.Close()

	_, err = smt.Exec(usr.ID, pathA)
	if err != nil {
		return err
	}

	return nil
}

// ArchiveChannel archives a channel's messages.
func (a *Archiver) ArchiveChannel(s *discordgo.Session, tx *sql.Tx, channelID string, opt *Options) error {
	if opt == nil {
		opt = NewOptions()
	}
	err := a.InitDB(tx, opt)
	if err != nil {
		return err
	}

	// Obtain channel and guild information
	channel, err := s.Channel(channelID)
	if err != nil {
		return err
	}

	err = a.InsertChannel(tx, channel)
	if err != nil {
		return err
	}

	guild, err := s.Guild(channel.GuildID)
	if err != nil {
		return err
	}

	err = a.InsertGuild(tx, guild)
	if err != nil {
		return err
	}

	// Archive channel messages
	var lastID string

	// Skip n messages
	if opt.Skip > 0 {
		msg, err := nthChannelMessage(s, channelID, opt.Skip)
		if err != nil {
			a.logf("[error] error skipping [%d] messages in channel [%s]: %s", opt.Skip, channel.Name, err.Error())
			return err
		}
		a.logf("[info] skipped [%d] messages in channel [%s]. beforeID[%s]", opt.Skip, channel.Name, msg.ID)
		lastID = msg.ID
	} else {
		lastID = opt.LastID
	}

	var numArchived int
	for i := 0; ; i++ {
		// Number of messages to fetch
		var fetchnum int
		if opt.Limit > 0 {
			if numArchived >= opt.Limit {
				a.logf("[info] reached message limit [%d].", opt.Limit)
				return nil
			}

			fetchnum = opt.Limit - numArchived
			if fetchnum > 100 {
				fetchnum = 100
			}
		} else {
			fetchnum = 100
		}

		msgs, err := s.ChannelMessages(channelID, fetchnum, lastID, "", "")
		if err != nil {
			return err
		}
		if len(msgs) == 0 {
			return nil
		}

		numArchived += len(msgs)

		// Insert messages into database
		for _, msg := range msgs {
			err := a.InsertMessage(s, guild.ID, tx, msg, opt)
			if err != nil {
				return err
			}

			if opt.SaveAttachments {
				<-a.downloadTokens
				go func(msg *discordgo.Message) {
					err = a.downloadAttachments(tx, msg)
					if err != nil {
						a.logf("[error] error downloading attachments for message [%s] in channel [%s]: %s", msg.ID, channel.Name, err.Error())
					}
					a.downloadTokens <- struct{}{}
				}(msg)
			}
			if opt.SaveEmbedImages {
				<-a.downloadTokens
				go func(msg *discordgo.Message) {
					err = a.downloadEmbeds(tx, msg)
					if err != nil {
						a.logf("[error] error downloading embeds for message [%s] in channel [%s]: %s", msg.ID, channel.Name, err.Error())
					}
					a.downloadTokens <- struct{}{}
				}(msg)
			}
		}

		a.logf("[info] archived [%d] messages in channel [%s] lastID[%s]", numArchived, channel.Name, lastID)
		lastID = msgs[len(msgs)-1].ID
	}
}

// ArchiveGuild archives all the channels in a guild
func (a *Archiver) ArchiveGuild(s *discordgo.Session, tx *sql.Tx, guildID string, opt *Options) error {
	if opt == nil {
		opt = NewOptions()
	}

	channels, err := s.GuildChannels(guildID)
	if err != nil {
		return err
	}

	for _, channel := range channels {
		if channel.Type == discordgo.ChannelTypeGuildText {
			a.logf("[info] archiving channel [%s] - [%s]", channel.Name, channel.Topic)
			err = a.ArchiveChannel(s, tx, channel.ID, opt)
			if err != nil {
				a.log("[error] error archiving channel: ", err)
			}
		}
	}
	return nil
}

// ArchiveMembers archives the members of a guild
func (a *Archiver) ArchiveMembers(s *discordgo.Session, tx *sql.Tx, guildID string, opt *Options) error {
	err := a.InitDB(tx, nil)
	if err != nil {
		return err
	}

	var lastID string
	if opt.Skip > 0 {
		m, err := nthGuildMember(s, guildID, opt.Skip)
		if err != nil {
			return err
		}
		lastID = m.User.ID
	} else {
		lastID = opt.LastID
	}

	var count int
	// Request guild member info in chunks of 1000.
	for {
		members, err := s.GuildMembers(guildID, lastID, 1000)
		if err != nil {
			return err
		}
		if len(members) == 0 {
			break
		}
		if opt.Limit > 0 && count > opt.Limit {
			return nil
		}

		for _, m := range members {
			m.GuildID = guildID
			err = a.InsertMember(tx, m)
			if err != nil {
				return err
			}
			err = a.InsertUser(tx, m.User)
			if err != nil {
				return err
			}

			if opt.SaveAvatars {
				<-a.downloadTokens
				go func(m *discordgo.Member) {
					err := a.downloadAvatar(tx, m.User, opt)
					if err != nil {
						a.logf("[error] error downloading avatar for user [%s]: %s", m.User.Username, err.Error())
					}
					a.downloadTokens <- struct{}{}
				}(m)
			}
		}

		count += len(members)
		lastID = members[len(members)-1].User.ID
	}

	return nil
}
