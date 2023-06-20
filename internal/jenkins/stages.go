package jenkins

import (
	"fmt"
	"net/http"
	"net/url"
	"time"
)

type respWFAPIRaw struct {
	Stages []*respWFAPIStageRaw `json:"stages"`
}

type respWFAPIStageRaw struct {
	Name           string `json:"name"`
	Status         string `json:"status"`
	DurationMillis int64  `json:"durationMillis"`
}

type Stage struct {
	Name     string
	Status   string
	Duration time.Duration
}

func respWFAPIRawToStage(raw *respWFAPIStageRaw) *Stage {
	return &Stage{
		Name:     raw.Name,
		Status:   raw.Status,
		Duration: time.Duration(raw.DurationMillis) * time.Millisecond,
	}
}

func respWFAPIRawToStages(resp *respWFAPIRaw) []*Stage {
	res := make([]*Stage, len(resp.Stages))
	for i, stage := range resp.Stages {
		res[i] = respWFAPIRawToStage(stage)
	}
	return res
}

func (c *Client) wfapiJobBuildURL(folderName, jobName, branchName, buildID string) (string, error) {
	if jobName == "" {
		return url.JoinPath(c.serverURL, "job", folderName, buildID, "wfapi")
	}

	if branchName == "" {
		return url.JoinPath(c.serverURL, "job", folderName, "job", jobName, buildID, "wfapi")
	}

	return url.JoinPath(c.serverURL, "job", folderName, "job", jobName, "job", branchName, buildID, "wfapi")
}

func (c *Client) Stages(folderName, jobName, branchName string, buildID int64) ([]*Stage, error) {
	var resp respWFAPIRaw

	wfapiURL, err := c.wfapiJobBuildURL(folderName, jobName, branchName, fmt.Sprint(buildID))
	if err != nil {
		return nil, err
	}

	err = c.do(http.MethodGet, wfapiURL, &resp)
	if err != nil {
		return nil, err
	}

	return respWFAPIRawToStages(&resp), nil
}
