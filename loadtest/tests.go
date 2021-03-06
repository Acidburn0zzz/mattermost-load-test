// Copyright (c) 2017 Mattermost, Inc. All Rights Reserved.
// See License.txt for license information.

package loadtest

import (
	"encoding/json"
	"io"
	"io/ioutil"
	"math/rand"
	"net/http"
	"os"
	"strconv"

	"bytes"

	"github.com/icrowley/fake"
	"github.com/mattermost/mattermost-load-test/cmdlog"
	"github.com/mattermost/mattermost-load-test/randutil"
	"github.com/mattermost/platform/model"
	"github.com/mattermost/platform/utils"
)

type TestRun struct {
	UserEntities []UserEntityFrequency
}

type UserEntityFrequency struct {
	Freq           float64
	RateMultiplier float64
	Entity         UserEntity
}

type UserEntity struct {
	Name    string
	Actions []randutil.Choice
}

type EntityActions interface {
	Init(c *EntityConfig)
	Action(c *EntityConfig)
}

func readTestFile(name string) ([]byte, error) {
	path, _ := utils.FindDir("testfiles")
	file, err := os.Open(path + "/" + name)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	data := &bytes.Buffer{}
	if _, err := io.Copy(data, file); err != nil {
		return nil, err
	} else {
		return data.Bytes(), nil
	}
}

func readRandomTestFile() ([]byte, error, string) {
	path, _ := utils.FindDir("testfiles")
	files, err := ioutil.ReadDir(path)
	if err != nil {
		panic("Can't read testfiles directory.")
	}

	fileI := rand.Intn(len(files))
	file := files[fileI]
	for file.IsDir() {
		fileI = rand.Intn(len(files))
		file = files[fileI]
	}

	b, err := readTestFile(file.Name())
	return b, err, file.Name()
}

func actionGetStatuses(c *EntityConfig) {
	idsI, ok := c.Info["statusUserIds"+c.UserData.Username]
	var ids []string
	if !ok {
		team, channel := c.UserData.PickTeamChannel()
		if team == nil || channel == nil {
			return
		}
		channelId := c.ChannelMap[team.Name+channel.Name]

		if channelId == "" {
			cmdlog.Error("Unable to get channel from map")
			return
		}

		members, resp := c.Client.GetChannelMembers(channelId, 0, 60, "")
		if resp.Error != nil {
			cmdlog.Errorf("Unable to get members for channel %v to seed action get status. Error: %v", channelId, resp.Error.Error())
		}

		ids = make([]string, len(*members), len(*members))
		for i := 0; i < len(*members); i++ {
			ids[i] = (*members)[i].UserId
		}

		c.Info["statusUserIds"+c.UserData.Username] = ids
	} else {
		ids = idsI.([]string)
	}

	if _, resp := c.Client.GetUsersStatusesByIds(ids); resp.Error != nil {
		cmdlog.Error("Unable to get user statuses by Ids. Error: " + resp.Error.Error())
	}
}

func actionPost(c *EntityConfig) {
	team, channel := c.UserData.PickTeamChannel()
	if team == nil || channel == nil {
		return
	}
	channelId := c.ChannelMap[team.Name+channel.Name]

	if channelId == "" {
		cmdlog.Error("Unable to get channel from map")
		return
	}

	post := &model.Post{
		ChannelId: channelId,
		Message:   fake.Sentences(),
	}

	if rand.Float64() < c.LoadTestConfig.UserEntitiesConfiguration.UploadImageChance {
		numFiles := rand.Intn(3) + 1
		fileIds := make([]string, numFiles, numFiles)
		for i := 0; i < numFiles; i++ {
			if data, err, filename := readRandomTestFile(); err != nil {
				cmdlog.Errorf("Problem reading test file. Error %v", err.Error())
			} else {
				if file, resp := c.Client.UploadFile(data, channelId, filename); resp.Error != nil {
					cmdlog.Error("Unable to upload file. Error: " + resp.Error.Error())
					return
				} else {
					fileIds[i] = file.FileInfos[0].Id
				}
			}
		}
		post.FileIds = fileIds
	}

	_, resp := c.Client.CreatePost(post)
	if resp.Error != nil {
		cmdlog.Infof("Failed to post to team %v on channel %v as user %v with token %v. Error: %v", team.Name, channel.Name, c.UserData.Username, c.Client.AuthToken, resp.Error.Error())
	}
}

func actionPost3(c *EntityConfig) {
	team, channel := c.UserData.PickTeamChannel()
	if team == nil || channel == nil {
		return
	}
	channelId := c.ChannelMap[team.Name+channel.Name]
	teamId := c.TeamMap[team.Name]

	if channelId == "" {
		cmdlog.Error("Unable to get channel from map")
		return
	}
	if teamId == "" {
		cmdlog.Error("Unable to get team from map")
		return
	}

	post := &model.Post{
		ChannelId: channelId,
		Message:   fake.Sentences(),
	}

	c.Client3.SetTeamId(teamId)
	if rand.Float64() < c.LoadTestConfig.UserEntitiesConfiguration.UploadImageChance {
		numFiles := rand.Intn(3) + 1
		fileIds := make([]string, numFiles, numFiles)
		for i := 0; i < numFiles; i++ {
			if data, err, filename := readRandomTestFile(); err != nil {
				cmdlog.Errorf("Problem reading test file. Error %v", err.Error())
			} else {
				if file, err := c.Client3.UploadPostAttachment(data, channelId, filename); err != nil {
					cmdlog.Error("Unable to upload file. Error: " + err.Error())
					return
				} else {
					fileIds[i] = file.FileInfos[0].Id
				}
			}
		}
		post.FileIds = fileIds
	}

	_, err := c.Client3.CreatePost(post)
	if err != nil {
		cmdlog.Infof("Failed to post to team %v on channel %v as user %v with token %v. Error: %v", team.Name, channel.Name, c.UserData.Username, c.Client3.AuthToken, err.Error())
	}
}

func actionGetChannel(c *EntityConfig) {
	team, channel := c.UserData.PickTeamChannel()
	if team == nil || channel == nil {
		return
	}
	channelId := c.ChannelMap[team.Name+channel.Name]
	teamId := c.TeamMap[team.Name]

	c.Client3.SetTeamId(teamId)

	if _, resp := c.Client.ViewChannel("me", &model.ChannelView{
		ChannelId:     channelId,
		PrevChannelId: "",
	}); resp.Error != nil {
		cmdlog.Errorf("Unable to view channel. Channel: %v, User: %v", channelId, c.UserData.Username)
	}

	if _, resp := c.Client.GetChannelMember(channelId, "me", ""); resp.Error != nil {
		cmdlog.Errorf("Unable to get channel member. Channel: %v, User: %v", channelId, c.UserData.Username)
	}

	if _, resp := c.Client.GetChannelMembers(channelId, 0, 60, ""); resp.Error != nil {
		cmdlog.Errorf("Unable to get channel member. Channel: %v, User: %v", channelId, c.UserData.Username)
	}

	if _, resp := c.Client.GetChannelStats(channelId, ""); resp.Error != nil {
		cmdlog.Errorf("Unable to get channel stats. Channel: %v, User: %v", channelId, c.UserData.Username)
	}

	if posts, resp := c.Client.GetPostsForChannel(channelId, 0, 60, ""); resp.Error != nil {
		cmdlog.Errorf("Unable to get posts for channel Channel: %v, User: %v", channelId, c.UserData.Username)
	} else {
		for _, post := range posts.Posts {
			if post.HasReactions {
				if _, err := c.Client3.ListReactions(channelId, post.Id); resp.Error != nil {
					cmdlog.Errorf("Unable to get reactions for post. Channel: %v, User: %v, Post: %v, Error: %v", channelId, c.UserData.Username, post.Id, err.Error())
				}
			}
			if len(post.FileIds) > 0 {
				if files, err := c.Client3.GetFileInfosForPost(channelId, post.Id, ""); resp.Error != nil {
					cmdlog.Errorf("Unable to get file infos for post. Channel: %v, User: %v, Post: %v, Error: %v", channelId, c.UserData.Username, post.Id, err.Error())
				} else {
					for _, file := range files {
						if file.IsImage() {
							if _, err := c.Client3.GetFileThumbnail(file.Id); err != nil {
								cmdlog.Errorf("Unable to get file thumbnail for file. Channel: %v, User: %v, Post: %v, File: %v, Error: %v", channelId, c.UserData.Username, post.Id, file.Id, err.Error())
							}
						}
					}
				}
			}
		}
	}
}

func actionPerformSearch(c *EntityConfig) {
	team, _ := c.UserData.PickTeamChannel()
	if team == nil {
		return
	}
	teamId := c.TeamMap[team.Name]

	_, resp := c.Client.SearchPosts(teamId, fake.Words(), false)
	if resp.Error != nil {
		cmdlog.Errorf("Failed to search: %v", resp.Error.Error())
	}
}

func actionDisconnectWebsocket(c *EntityConfig) {
	c.WebSocketClient.Close()
}

func actionPostWebhook(c *EntityConfig) {
	infokey := "webhookid" + strconv.Itoa(c.EntityNumber)
	hookIdI, ok := c.Info[infokey]
	hookId := ""
	if !ok {
		team, channel := c.UserData.PickTeamChannel()
		if team == nil || channel == nil {
			return
		}
		channelId := c.ChannelMap[team.Name+channel.Name]

		webhook, resp := c.Client.CreateIncomingWebhook(&model.IncomingWebhook{
			ChannelId:   channelId,
			DisplayName: model.NewId(),
			Description: model.NewId(),
		})
		if resp.Error != nil {
			cmdlog.Error("Unable to create incoming webhook. Error: " + resp.Error.Error())
			return
		}
		c.Info[infokey] = webhook.Id
		hookId = webhook.Id
	} else {
		hookId = hookIdI.(string)
	}

	webhookRequest := &model.IncomingWebhookRequest{
		Text:     fake.Paragraphs(),
		Username: "ltwhuser",
		Type:     "",
	}
	b, err := json.Marshal(webhookRequest)
	if err != nil {
		cmdlog.Error("Unable to marshal json for webhook request")
		return
	}

	var buf bytes.Buffer
	buf.WriteString(string(b))

	if _, err := http.Post(c.LoadTestConfig.ConnectionConfiguration.ServerURL+"/hooks/"+hookId, "application/json", &buf); err != nil {
		cmdlog.Error("Failed to post by webhook. Error: " + err.Error())
	}
}

var posterEntity UserEntity = UserEntity{
	Name: "Poster",
	Actions: []randutil.Choice{
		{
			Item:   actionPost,
			Weight: 50,
		},
		{
			Item:   actionPost3,
			Weight: 50,
		},
	},
}

var TestBasicPosting TestRun = TestRun{
	UserEntities: []UserEntityFrequency{
		{
			Freq:           100.0,
			RateMultiplier: 1.0,
			Entity:         posterEntity,
		},
	},
}

var getChannelEntity UserEntity = UserEntity{
	Name: "Get Channel",
	Actions: []randutil.Choice{
		{
			Item:   actionGetChannel,
			Weight: 1,
		},
	},
}

var TestGetChannel TestRun = TestRun{
	UserEntities: []UserEntityFrequency{
		{
			Freq:           100.0,
			RateMultiplier: 1.0,
			Entity:         getChannelEntity,
		},
	},
}

var searchEntity UserEntity = UserEntity{
	Name: "Search",
	Actions: []randutil.Choice{
		{
			Item:   actionPerformSearch,
			Weight: 1,
		},
	},
}

var TestSearch TestRun = TestRun{
	UserEntities: []UserEntityFrequency{
		{
			Freq:           100.0,
			RateMultiplier: 1.0,
			Entity:         searchEntity,
		},
	},
}

var standardUserEntity UserEntity = UserEntity{
	Name: "Standard",
	Actions: []randutil.Choice{
		{
			Item:   actionPost,
			Weight: 4,
		},
		{
			Item:   actionPerformSearch,
			Weight: 1,
		},
		{
			Item:   actionGetChannel,
			Weight: 28,
		},
		{
			Item:   actionDisconnectWebsocket,
			Weight: 2,
		},
	},
}

var webhookUserEntity UserEntity = UserEntity{
	Name: "Webhook",
	Actions: []randutil.Choice{
		{
			Item:   actionPostWebhook,
			Weight: 1,
		},
	},
}

var TestAll TestRun = TestRun{
	UserEntities: []UserEntityFrequency{
		{
			Freq:           90.0,
			RateMultiplier: 1.0,
			Entity:         standardUserEntity,
		},
		{
			Freq:           10.0,
			RateMultiplier: 1.5,
			Entity:         webhookUserEntity,
		},
	},
}
