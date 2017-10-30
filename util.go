package discordarchive

import (
	"errors"

	"github.com/bwmarrin/discordgo"
)

// Errors
var (
	ErrEmpty = errors.New("error: result empty")
)

// returns the nth message from a channel
func nthChannelMessage(s *discordgo.Session, channelID string, n int) (*discordgo.Message, error) {
	toSkip := n

	var beforeID string
	for {
		fetchnum := toSkip
		if fetchnum > 100 {
			fetchnum = 100
		}
		msgs, err := s.ChannelMessages(channelID, fetchnum, beforeID, "", "")
		if err != nil {
			return nil, err
		}
		if len(msgs) == 0 {
			return nil, ErrEmpty
		}

		toSkip -= len(msgs)
		if toSkip <= 0 {
			return msgs[len(msgs)-1], nil
		}

		beforeID = msgs[len(msgs)-1].ID
	}

}

func nthGuildMember(s *discordgo.Session, guildID string, n int) (*discordgo.Member, error) {
	toSkip := n

	var lastID string
	for {
		fetchnum := toSkip
		if fetchnum > 1000 {
			fetchnum = 1000
		}
		usrs, err := s.GuildMembers(guildID, lastID, fetchnum)
		if err != nil {
			return nil, err
		}
		if len(usrs) == 0 {
			return nil, ErrEmpty
		}

		toSkip -= len(usrs)
		if toSkip <= 0 {
			return usrs[len(usrs)-1], nil
		}

		lastID = usrs[len(usrs)-1].User.ID
	}
}
