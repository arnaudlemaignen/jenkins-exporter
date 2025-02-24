package jenkins

import (
	"errors"
	"fmt"
	"strconv"
	"time"
)

// https://github.com/jenkinsci/metrics-plugin/blob/master/src/main/java/jenkins/metrics/impl/TimeInQueueAction.java#L85
type actionRawResp struct {
	Class                  string `json:"_class"`
	WaitingTimeMillis      int64  `json:"waitingTimeMillis"`
	BuildableTimeMillis    int64  `json:"buildableTimeMillis"`
	BlockedTimeMillis      int64  `json:"blockedTimeMillis"`
	ExecutingTimeMillis    int64  `json:"executingTimeMillis"`
	BuildingDurationMillis int64  `json:"buildingDurationMillis"`
}
type buildRawResp struct {
	ID      string           `json:"id"`
	Actions []*actionRawResp `json:"actions"`
	Result  string           `json:"result"`
}

type jobRawResp struct {
	Name              string          `json:"name"`
	WorkflowJobBuilds []*buildRawResp `json:"builds"`
	MultiBranchJobs   []*jobRawResp   `json:"jobs"`
}

type respRaw struct {
	Jobs []*jobRawResp `json:"jobs"`
}

type Build struct {
	FolderName       string
	JobName          string
	BranchName       string
	SubBranchName    string
	ID               int64
	BuildableTime    time.Duration
	WaitingTime      time.Duration
	BlockedTime      time.Duration
	ExecutingTime    time.Duration
	BuildingDuration time.Duration
	Result           string
}

func (c *Client) buildRawToBuild(folderName, jobName, branchName, subBranchName string, rawBuild *buildRawResp) (*Build, error) {
	const metricClass = "jenkins.metrics.impl.TimeInQueueAction"

	for _, a := range rawBuild.Actions {
		if a.Class != metricClass {
			continue
		}

		intID, err := strconv.Atoi(rawBuild.ID)
		if err != nil {
			return nil, fmt.Errorf("could not convert id '%s' to int64", rawBuild.ID)
		}
		b := Build{
			FolderName:       folderName,
			JobName:          jobName,
			BranchName:       branchName,
			SubBranchName:    subBranchName,
			ID:               int64(intID),
			BuildableTime:    time.Duration(a.BuildableTimeMillis) * time.Millisecond,
			WaitingTime:      time.Duration(a.WaitingTimeMillis) * time.Millisecond,
			BlockedTime:      time.Duration(a.BlockedTimeMillis) * time.Millisecond,
			ExecutingTime:    time.Duration(a.ExecutingTimeMillis) * time.Millisecond,
			BuildingDuration: time.Duration(a.BuildingDurationMillis) * time.Millisecond,
			Result:           rawBuild.Result,
		}

		return &b, nil
	}

	return nil, errors.New("could not find metrics in Actions slice")
}

func buildIsInProgress(b *buildRawResp) bool {
	return b.Result == ""
}

func (c *Client) respRawToBuilds(raw *respRaw, removeInProgressBuilds bool) []*Build {
	var res []*Build

	for _, job := range raw.Jobs {

		for _, rawBuild := range job.WorkflowJobBuilds {
			if removeInProgressBuilds && buildIsInProgress(rawBuild) {
				continue
			}

			b, err := c.buildRawToBuild(job.Name, "", "", "", rawBuild)
			if err != nil {
				c.logger.Printf("skipping build %s/%s: %s", job.Name, rawBuild.ID, err)
				continue
			}

			res = append(res, b)
		}

		for _, multibranchJob := range job.MultiBranchJobs {
			//jobs
			for _, rawBuild := range multibranchJob.WorkflowJobBuilds {
				if removeInProgressBuilds && buildIsInProgress(rawBuild) {
					continue
				}

				b, err := c.buildRawToBuild(job.Name, multibranchJob.Name, "", "", rawBuild)
				if err != nil {
					c.logger.Printf("skipping build %s/%s/%s: %s", job.Name, multibranchJob.Name, rawBuild.ID, err)
					continue
				}

				res = append(res, b)
			}

			//multibranches Level 3
			for _, multibranchJobChild := range multibranchJob.MultiBranchJobs {
				for _, rawBuild := range multibranchJobChild.WorkflowJobBuilds {
					if removeInProgressBuilds && buildIsInProgress(rawBuild) {
						continue
					}

					b, err := c.buildRawToBuild(job.Name, multibranchJob.Name, multibranchJobChild.Name, "", rawBuild)
					if err != nil {
						c.logger.Printf("skipping build %s/%s/%s/%s: %s", job.Name, multibranchJob.Name, multibranchJobChild.Name, rawBuild.ID, err)
						continue
					}

					res = append(res, b)
				}

				//multibranches Level 4
				for _, multibranchJobSubChild := range multibranchJobChild.MultiBranchJobs {
					for _, rawBuild := range multibranchJobSubChild.WorkflowJobBuilds {
						if removeInProgressBuilds && buildIsInProgress(rawBuild) {
							continue
						}

						b, err := c.buildRawToBuild(job.Name, multibranchJob.Name, multibranchJobChild.Name, multibranchJobSubChild.Name, rawBuild)
						if err != nil {
							c.logger.Printf("skipping build %s/%s/%s/%s/%s: %s", job.Name, multibranchJob.Name, multibranchJobChild.Name, multibranchJobSubChild.Name, rawBuild.ID, err)
							continue
						}

						res = append(res, b)
					}
				}

			}

		}

	}

	return res
}

func (c *Client) Builds(inProgressBuilds bool) ([]*Build, error) {
	// TODO: is it possible to retrieve only the element in actions with
	// _class = "jenkins.metrics.impl.TimeInQueueAction" that contains the
	// metrics?
	const queryBuilds = "builds[id,result,actions[_class,buildableTimeMillis,waitingTimeMillis,blockedTimeMillis,executingTimeMillis,buildingDurationMillis]]"

	//for debugging, works with all level
	//const endpoint = "api/json?tree=jobs[name,builds[id,result],jobs[name,builds[id,result],jobs[name,builds[id,result]]]]"
	//Manage up to 3 nested job tree e.g :
	//https://jenkins.EXAMPLE.com/job/JOB_EXAMPLE/job/PROJECT_EXAMPLE/job/BRANCH_EXAMPLE/job/SUBBRANCH_EXAMPLE/
	const endpoint = "api/json?tree=jobs[name," + queryBuilds +
		",jobs[name," + queryBuilds + ",jobs[name," + queryBuilds + ",jobs[name," + queryBuilds + "]]]]"

	var resp respRaw
	err := c.do("GET", c.serverURL+endpoint, &resp)
	if err != nil {
		return nil, err
	}

	builds := c.respRawToBuilds(&resp, !inProgressBuilds)

	return builds, nil
}
