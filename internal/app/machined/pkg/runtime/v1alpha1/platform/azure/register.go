// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package azure

import (
	"bytes"
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/url"

	"github.com/siderolabs/talos/pkg/download"
)

// This should provide the bare minimum to trigger a node in ready condition to allow
// azure to be happy with the node and let it on it's lawn.
func linuxAgent(ctx context.Context) (err error) {
	var gs *GoalState

	gs, err = goalState(ctx)
	if err != nil {
		return fmt.Errorf("failed to register with Azure and fetch GoalState XML: %w", err)
	}

	return reportHealth(ctx, gs.Incarnation, gs.Container.ContainerID, gs.Container.RoleInstanceList.RoleInstance.InstanceID)
}

func goalState(ctx context.Context) (gs *GoalState, err error) {
	body, err := download.Download(ctx, AzureInternalEndpoint+"/machine/?comp=goalstate",
		download.WithHeaders(map[string]string{
			"x-ms-agent-name": "WALinuxAgent",
			"x-ms-version":    "2015-04-05",
			"Content-Type":    "text/xml;charset=utf-8",
		}))
	if err != nil {
		return nil, err
	}

	gs = &GoalState{}
	err = xml.Unmarshal(body, gs)

	return gs, err
}

func reportHealth(ctx context.Context, gsIncarnation, gsContainerID, gsInstanceID string) (err error) {
	// Construct health response
	h := &Health{
		Xsi: "http://www.w3.org/2001/XMLSchema-instance",
		Xsd: "http://www.w3.org/2001/XMLSchema",
		WAAgent: WAAgent{
			GoalStateIncarnation: gsIncarnation,
			Container: &Container{
				ContainerID: gsContainerID,
				RoleInstanceList: &RoleInstanceList{
					Role: &RoleInstance{
						InstanceID: gsInstanceID,
						Health: &HealthStatus{
							State: "Ready",
						},
					},
				},
			},
		},
	}

	// Encode health response as xml
	b := new(bytes.Buffer)
	b.WriteString(xml.Header)

	err = xml.NewEncoder(b).Encode(h)
	if err != nil {
		return err
	}

	var u *url.URL

	u, err = url.Parse(AzureInternalEndpoint + "/machine/?comp=health")
	if err != nil {
		return nil
	}

	var (
		req  *http.Request
		resp *http.Response
	)

	req, err = http.NewRequestWithContext(ctx, http.MethodPost, u.String(), b)
	if err != nil {
		return err
	}

	addHeaders(req)

	client := &http.Client{}

	resp, err = client.Do(req)
	if err != nil {
		return err
	}

	// TODO probably should do some better check here ( verify status code )
	//nolint:errcheck
	defer resp.Body.Close()

	_, err = io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	return err
}

func addHeaders(req *http.Request) {
	req.Header.Add("X-Ms-Agent-Name", "WALinuxAgent")
	req.Header.Add("X-Ms-Version", "2015-04-05")
	req.Header.Add("Content-Type", "text/xml;charset=utf-8")
}

// GoalState is the response from the Azure platform when a machine
// starts up. Ref:
// https://github.com/Azure/WALinuxAgent/blob/b26feb7822f7d4a19507b6762fe1bd280c2ba2de/bin/waagent2.0#L4331
// https://github.com/Azure/WALinuxAgent/blob/3be3e1fbf2330303f76961b87d891672e847ce4e/azurelinuxagent/common/protocol/wire.py#L216
type GoalState struct {
	XMLName xml.Name `xml:"GoalState"`
	Xsi     string   `xml:"xsi,attr"`
	Xsd     string   `xml:"xsd,attr"`
	WAAgent
}

// Health is the response from the local machine to Azure to denote current
// machine state.
type Health struct {
	XMLName xml.Name `xml:"Health"`
	Xsi     string   `xml:"xmlns:xsi,attr"`
	Xsd     string   `xml:"xmlns:xsd,attr"`
	WAAgent
}

// WAAgent contains the meat of the data format that is passed between the
// Azure platform and the machine.
// Mostly, we just care about the Incarnation and Container fields here.
type WAAgent struct {
	Text                 string     `xml:",chardata"`
	Version              string     `xml:"Version,omitempty"`
	Incarnation          string     `xml:"Incarnation,omitempty"`
	GoalStateIncarnation string     `xml:"GoalStateIncarnation,omitempty"`
	Machine              *Machine   `xml:"Machine,omitempty"`
	Container            *Container `xml:"Container,omitempty"`
}

// Container holds the interesting details about a provisioned machine.
type Container struct {
	Text             string            `xml:",chardata"`
	ContainerID      string            `xml:"ContainerId"`
	RoleInstanceList *RoleInstanceList `xml:"RoleInstanceList"`
}

// RoleInstanceList is a list but only has a single item which is cool I guess.
type RoleInstanceList struct {
	Text         string        `xml:",chardata"`
	RoleInstance *RoleInstance `xml:"RoleInstance,omitempty"`
	Role         *RoleInstance `xml:"Role,omitempty"`
}

// RoleInstance contains the specifics for the provisioned VM.
type RoleInstance struct {
	Text          string         `xml:",chardata"`
	InstanceID    string         `xml:"InstanceId"`
	State         string         `xml:"State,omitempty"`
	Configuration *Configuration `xml:"Configuration,omitempty"`
	Health        *HealthStatus  `xml:"Health,omitempty"`
}

// Configuration seems important but isnt really used right now. We could
// very well not include it because we have no use for it right now, but
// since we want completeness, we're going to include it.
type Configuration struct {
	Text                     string `xml:",chardata"`
	HostingEnvironmentConfig string `xml:"HostingEnvironmentConfig"`
	SharedConfig             string `xml:"SharedConfig"`
	ExtensionsConfig         string `xml:"ExtensionsConfig"`
	FullConfig               string `xml:"FullConfig"`
	Certificates             string `xml:"Certificates"`
	ConfigName               string `xml:"ConfigName"`
}

// Machine holds no useful information for us.
type Machine struct {
	Text                  string `xml:",chardata"`
	ExpectedState         string `xml:"ExpectedState"`
	StopRolesDeadlineHint string `xml:"StopRolesDeadlineHint"`
	LBProbePorts          *struct {
		Text string `xml:",chardata"`
		Port string `xml:"Port"`
	} `xml:"LBProbePorts,omitempty"`
	ExpectHealthReport string `xml:"ExpectHealthReport"`
}

// HealthStatus provides mechanism to trigger Azure to understand that our
// machine has transitioned to a 'Ready' state and is good to go.
// We can fill out details if we want to be more verbose...
type HealthStatus struct {
	Text    string `xml:",chardata"`
	State   string `xml:"State"`
	Details *struct {
		Text        string `xml:",chardata"`
		SubStatus   string `xml:"SubStatus"`
		Description string `xml:"Description"`
	} `xml:"Details,omitempty"`
}
