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
		mentions    string
	)

	msg := &discordgo.Message{}
	msg.Author = &discordgo.User{}

	err := row.Scan(
		&msg.ChannelID,
		&msg.ID,
		&msg.Author.ID,
		&msg.Author.Username,
		&msg.Author.Avatar,
		&msg.Content,
		&mentions,
		&embeds,
		&attachments)

	if err != nil {
		return nil, err
	}

	err = json.Unmarshal([]byte(mentions), &msg.Mentions)
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

// ScanGuilds ...
func ScanGuilds(rows *sql.Rows) ([]*discordgo.Guild, error) {
	guilds := []*discordgo.Guild{}
	for rows.Next() {
		g, err := scanGuild(rows)
		if err != nil {
			return nil, err
		}
		guilds = append(guilds, g)
	}
	return guilds, nil
}

// scanGuild ...
func scanGuild(rows *sql.Rows) (*discordgo.Guild, error) {
	var (
		guildID   string
		name      string
		guildJSON string
		guild     = &discordgo.Guild{}
	)

	err := rows.Scan(&guildID, &name, &guildJSON)
	if err != nil {
		return nil, err
	}

	err = json.Unmarshal([]byte(guildJSON), guild)
	if err != nil {
		return nil, err
	}

	return guild, nil
}

// ScanChannels ...
func ScanChannels(rows *sql.Rows) ([]*discordgo.Channel, error) {
	channels := []*discordgo.Channel{}
	for rows.Next() {
		c, err := scanChannel(rows)
		if err != nil {
			return nil, err
		}
		channels = append(channels, c)
	}
	return channels, nil
}

func scanChannel(rows *sql.Rows) (*discordgo.Channel, error) {
	var (
		channelID   string
		guildID     string
		name        string
		topic       string
		t           int
		channelJSON string
	)

	err := rows.Scan(
		&channelID,
		&guildID,
		&name,
		&topic,
		&t,
		&channelJSON)

	channel := &discordgo.Channel{}
	err = json.Unmarshal([]byte(channelJSON), channel)
	if err != nil {
		return nil, err
	}

	return channel, nil
}
