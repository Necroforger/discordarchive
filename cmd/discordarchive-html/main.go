package main

import (
	"database/sql"
	"html/template"
	"log"
	"os"

	"github.com/Necroforger/discordarchive"
	"github.com/bwmarrin/discordgo"
	_ "github.com/mattn/go-sqlite3"
)

func handle(err error) {
	if err != nil {
		log.Println(err)
		panic(err)
	}
}

type content struct {
	Page     int
	Messages []*discordgo.Message
	Guilds   []*discordgo.Guild
	Channels []*discordgo.Channel
}

func createTemplate(db *sql.DB) *template.Template {
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
		"getguildsplash": func(guild *discordgo.Guild) string {
			return ""
		},
	})
	_, err := tmpl.ParseGlob("tmpl/*.html")
	handle(err)

	return tmpl
}

func main() {
	f, err := os.OpenFile("output.html", os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
	handle(err)
	defer f.Close()

	db, err := sql.Open("sqlite3", "archive.db")
	handle(err)
	defer db.Close()

	messages, err := discordarchive.ChannelMessages(db, "221341345539686400", 0, 100)
	handle(err)

	totalguilds, err := discordarchive.Guilds(db)
	handle(err)

	totalchannels, err := discordarchive.Channels(db, "221341345539686400")
	handle(err)

	tmpl := createTemplate(db)

	err = tmpl.ExecuteTemplate(f, "main", content{
		Page:     0,
		Messages: messages,
		Guilds:   totalguilds,
		Channels: totalchannels,
	})
	handle(err)
}
