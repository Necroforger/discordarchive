package main

import (
	"database/sql"
	"flag"
	"log"
	"os"
	"path/filepath"

	"github.com/Necroforger/discordarchive"
	"github.com/bwmarrin/discordgo"
	_ "github.com/mattn/go-sqlite3"
)

// Flags
var (
	OutPath         = flag.String("o", "./", "output folder")
	SaveEmbeds      = flag.Bool("embeds", false, "save images in embeds to files")
	SaveAttachments = flag.Bool("attachments", false, "save message attachments to files")
	SaveAvatars     = flag.Bool("avatars", false, "Save user avatars to files")
	AvatarSize      = flag.String("avatar-size", "", "Size of avatars when saving to file as a power of 2")
	ArchiveMembers  = flag.Bool("members", false, "Archive the members of a guild when archiving channels")
	Skip            = flag.Int("skip", 0, "number of messages to skip before archiving")
	Limit           = flag.Int("limit", 0, "maximum number of messages to archive")
	MethodGuild     = flag.Bool("g", false, "Save a guild or list of guilds")
	Token           = flag.String("t", "", "Discord token")
)

func main() {
	flag.Parse()

	args := flag.Args()
	if len(args) == 0 {
		log.Println("Please enter a target id")
	}

	session, err := discordgo.New(*Token)
	if err != nil {
		log.Println(err)
		return
	}
	err = session.Open()
	if err != nil {
		log.Println(err)
		return
	}

	os.MkdirAll(*OutPath, 0600)

	db, err := sql.Open("sqlite3", filepath.Join(*OutPath, "archive.db"))
	if err != nil {
		log.Println(err)
		return
	}
	defer db.Close()

	tx, err := db.Begin()
	if err != nil {
		log.Println(err)
		return
	}

	arc := discordarchive.New()
	arc.Log = os.Stderr
	arc.SavePath = *OutPath

	switch {
	// Archive guilds
	case *MethodGuild:
		for _, id := range args {
			err = arc.ArchiveGuild(session, tx, id, &discordarchive.Options{
				SaveAttachments: *SaveAttachments,
				SaveAvatars:     *SaveAvatars,
				SaveEmbedImages: *SaveEmbeds,
				Skip:            *Skip,
				Limit:           *Limit,
			})
			if err != nil {
				log.Println(err)
				return
			}
			err = arc.ArchiveMembers(session, tx, id, &discordarchive.Options{
				SaveAvatars: true,
				AvatarSize:  *AvatarSize,
			})
		}
		// Archive channels
	default:
		for _, id := range args {
			err = arc.ArchiveChannel(session, tx, id, &discordarchive.Options{
				SaveAttachments: *SaveAttachments,
				SaveAvatars:     *SaveAvatars,
				SaveEmbedImages: *SaveEmbeds,
				Skip:            *Skip,
				Limit:           *Limit,
			})
			if err != nil {
				log.Println(err)
				return
			}

			if *ArchiveMembers {
				channel, err := session.Channel(id)
				if err != nil {
					log.Println(err)
					return
				}

				err = arc.ArchiveMembers(session, tx, channel.GuildID, &discordarchive.Options{
					SaveAvatars: true,
					AvatarSize:  *AvatarSize,
				})
				if err != nil {
					log.Println(err)
					return
				}
			}
		}
	}

	err = tx.Commit()
	if err != nil {
		log.Println(err)
		return
	}
}
