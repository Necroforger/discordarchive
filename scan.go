package discordarchive

import (
	"database/sql"
	"encoding/json"

	"github.com/bwmarrin/discordgo"
)

// ScanMessages scans messages from a group of roles
func ScanMessages(rows *sql.Rows) ([]*discordgo.Message, error) {
	messages := []*discordgo.Message{}
	for rows.Next() {
		msg, err := scanMessage(rows)
		if err != nil {
			return nil, err
		}
		messages = append(messages, msg)
	}

	return messages, nil
}

func scanMessage(row *sql.Rows) (*discordgo.Message, error) {
	var (
		embeds      string
		attachments string
	)
	msg := &discordgo.Message{}
	err := row.Scan(&msg.ChannelID, &msg.ID, &msg.Author.ID, &msg.Author.Username, nil, &msg.Content, &embeds, &attachments)
	if err != nil {
		return nil, err
	}

	err = json.Unmarshal([]byte(embeds), &msg.Embeds)
	if err != nil {
		return nil, err
	}

	err = json.Unmarshal([]byte(attachments), &msg.Attachments)
	if err != nil {
		return nil, err
	}

	return msg, nil
}
