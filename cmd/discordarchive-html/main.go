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
}

func main() {

	f, err := os.OpenFile("output.html", os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
	handle(err)
	defer f.Close()

	db, err := sql.Open("sqlite3", "archive.db")
	handle(err)
	defer db.Close()

	rows, err := db.Query("SELECT * FROM messages ORDER BY messageID")
	handle(err)
	defer rows.Close()

	messages, err := discordarchive.ScanMessages(rows)
	handle(err)

	tmpl, err := template.New("tmpl.html").Funcs(template.FuncMap{
		"getavatar": func(usr *discordgo.User) string {
			return usr.AvatarURL("32")
		},
		"nickname": func(userid string) string {
			row := db.QueryRow("SELECT nickname FROM members WHERE userID=?", userid)
			nick := ""
			err = row.Scan(&nick)
			if err != nil {
				return ""
			}
			return nick
		},
	}).ParseFiles("./tmpl.html")
	handle(err)

	err = tmpl.Execute(f, content{
		Page:     0,
		Messages: messages,
	})
	handle(err)

}
