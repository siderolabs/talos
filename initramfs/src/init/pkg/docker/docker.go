package docker

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/docker/engine-api/client"
	"github.com/docker/engine-api/types"
)

type containerRow struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	State  string `json:"state"`
	Status string `json:"status"`
}

func ContainersHandleFunc(w http.ResponseWriter, r *http.Request) {
	cli, err := client.NewEnvClient()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	containers, err := cli.ContainerList(context.Background(), types.ContainerListOptions{})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	var rows []*containerRow
	for _, container := range containers {
		row := &containerRow{
			ID:     container.ID,
			Name:   container.Names[0],
			State:  container.State,
			Status: container.Status,
		}
		rows = append(rows, row)
	}

	js, err := json.Marshal(rows)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write(js)
}
