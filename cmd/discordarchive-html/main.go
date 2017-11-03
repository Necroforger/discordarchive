package main

import (
	"database/sql"
	"flag"
	"fmt"
	"html/template"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/Necroforger/discordarchive"

	"github.com/bwmarrin/discordgo"
	_ "github.com/mattn/go-sqlite3"
)

// Flags
var (
	DestPath = flag.String("o", "./", "set the destination path of the generated content")
	DBPath   = flag.String("i", "./archive.db", "set the database path")
)

// Content is the template content.
type Content struct {
	Page    int
	MaxPage int
	// Current channel
	Channel *discordgo.Channel
	// Current guild
	Guild *discordgo.Guild

	Messages []*discordgo.Message
	Guilds   []*discordgo.Guild
	Channels []*discordgo.Channel
}

func main() {
	flag.Parse()
	args := flag.Args()
	if len(args) > 0 {
		*DBPath = args[0]
	}

	db, err := sql.Open("sqlite3", *DBPath)
	handle(err)

	tmpl, err := createTemplate(db)
	handle(err)

	handle(generateAll(db, tmpl, *DestPath))
}

func handle(err error) {
	if err != nil {
		log.Println(err)
		panic(err)
	}
}

func generateAll(db *sql.DB, tmpl *template.Template, path string) error {
	os.MkdirAll(path, 0600)

	guilds, err := discordarchive.Guilds(db)
	if err != nil {
		return err
	}

	for _, guild := range guilds {
		err = generateGuild(db, tmpl, guild.ID, filepath.Join(path, guild.ID))
		if err != nil {
			return err
		}
	}

	return nil
}

func generateGuild(db *sql.DB, tmpl *template.Template, guildID, path string) error {
	os.MkdirAll(path, 0600)

	channels, err := discordarchive.Channels(db, guildID)
	if err != nil {
		return err
	}

	for _, channel := range channels {
		err = generateChannel(db, tmpl, channel.ID, filepath.Join(path, channel.ID))
		if err != nil {
			return err
		}
	}
	return nil
}

func generateChannel(db *sql.DB, tmpl *template.Template, channelID, path string) error {
	os.MkdirAll(path, 0600)

	cnt := &Content{}

	channel, err := discordarchive.Channel(db, channelID)
	if err != nil {
		return err
	}

	channels, err := discordarchive.Channels(db, channel.GuildID)
	if err != nil {
		return err
	}

	guild, err := discordarchive.Guild(db, channel.GuildID)
	if err != nil {
		return err
	}

	guilds, err := discordarchive.Guilds(db)
	if err != nil {
		return err
	}

	mCount, err := discordarchive.Count(db, "SELECT count(*) FROM messages WHERE channelID=?", channelID)
	if err != nil {
		return err
	}

	increment := 90

	cnt.Channels = channels
	cnt.Guilds = guilds
	cnt.Guild = guild
	cnt.Channel = channel
	cnt.MaxPage = mCount / increment

	for i := mCount; i > 0; i -= increment {
		limit := increment
		offset := i - increment
		if offset < 0 {
			offset = 0
		}

		messages, err := discordarchive.ChannelMessages(db, channelID, offset, limit)
		if err != nil {
			return err
		}

		cnt.Messages = messages
		cnt.Page = i / increment

		f, err := os.OpenFile(
			filepath.Join(
				path, fmt.Sprintf("%s-%d.html", channel.Name, cnt.Page),
			),
			os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600,
		)

		if err != nil {
			return err
		}
		defer f.Close()

		err = tmpl.ExecuteTemplate(f, "main", cnt)
		if err != nil {
			return err
		}
	}

	return nil
}

func createTemplate(db *sql.DB) (*template.Template, error) {
	tmpl := template.New("").Funcs(template.FuncMap{
		"getavatar": func(usr *discordgo.User) string {
			return usr.AvatarURL("32")
		},
		"getnickname": func(userid string) string {
			row := db.QueryRow("SELECT nickname FROM members WHERE userID=?", userid)
			nick := ""
			err := row.Scan(&nick)
			if err != nil {
				return ""
			}
			return nick
		},
		"getGuildSplash": func(guild *discordgo.Guild) string {
			return ""
		},
		"getChannelURL": func(channel *discordgo.Channel) string {
			return "../" + channel.ID + "/" + channel.Name + "-0.html"
		},
		"getGuildURL": func(guild *discordgo.Guild) string {
			channels, err := discordarchive.Channels(db, guild.ID)
			if err != nil {
				return ""
			}
			if len(channels) == 0 {
				return ""
			}
			return "../../" + guild.ID + "/" + channels[0].ID + "/" + channels[0].Name + "-0.html"
		},
		"getNextPage": func(cnt *Content, offset int) string {
			return cnt.Channel.Name + "-" + strconv.Itoa(cnt.Page+offset) + ".html"
		},
		"concat": func(dat ...interface{}) string {
			return fmt.Sprint(dat...)
		},
		"isImage": func(filepath string) bool {
			return strings.HasSuffix(filepath, "png") ||
				strings.HasSuffix(filepath, ".jpg") ||
				strings.HasSuffix(filepath, ".jpeg") ||
				strings.HasSuffix(filepath, ".gif") ||
				strings.HasSuffix(filepath, ".bmp")
		},
	})
	_, err := tmpl.ParseGlob("tmpl/*.html")
	if err != nil {
		return nil, err
	}

	return tmpl, nil
}
