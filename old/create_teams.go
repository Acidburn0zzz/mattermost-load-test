// Copyright (c) 2016 Mattermost, Inc. All Rights Reserved.
// See License.txt for license information.

package autocreation

import (
	"strconv"

	"github.com/mattermost/mattermost-load-test/loadtestconfig"
	"github.com/mattermost/platform/model"
)

type TeamsCreationResult struct {
	Teams  []*model.Team
	Errors []error
}

func CreateTeams(client *model.Client, config *loadtestconfig.TeamCreationConfiguration) *TeamsCreationResult {
	teamResults := &TeamsCreationResult{
		Teams:  make([]*model.Team, 0, config.Num),
		Errors: make([]error, 0, config.Num),
	}

	for teamNum := 1; teamNum <= config.Num; teamNum++ {
		team := &model.Team{
			Name:        config.Name + strconv.Itoa(teamNum),
			DisplayName: config.DisplayName + strconv.Itoa(teamNum),
			Type:        model.TEAM_OPEN,
		}

		if config.UseRandomId {
			team.Name = team.Name + model.NewId()
		}

		result, err := client.CreateTeam(team)
		if err != nil {
			teamResults.Errors = append(teamResults.Errors, err)
		} else {
			teamResults.Teams = append(teamResults.Teams, result.Data.(*model.Team))
		}
	}

	return teamResults
}

func (result *TeamsCreationResult) GetTeamIds() []string {
	teamIds := make([]string, 0, len(result.Teams))
	for _, team := range result.Teams {
		teamIds = append(teamIds, team.Id)
	}
	return teamIds
}
