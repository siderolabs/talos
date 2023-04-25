// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package talosctl provides the talosctl utility implementation.
package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/sashabaranov/go-openai"
	"github.com/siderolabs/gen/slices"
	"gopkg.in/yaml.v3"

	"github.com/siderolabs/talos/cmd/talosctl/cmd"
	"github.com/siderolabs/talos/pkg/cli"
	"github.com/siderolabs/talos/pkg/machinery/client/config"
	"github.com/siderolabs/talos/pkg/reporter"
	"github.com/siderolabs/talos/pkg/startup"
)

const (
	openAIMaxTokensEnvVar = "TALOSCTL_OPENAI_MAX_TOKENS"
)

type openaiInput struct {
	Args               []string
	Err                string
	Talosconfig        map[string]any
	TalosconfigReadErr error
}

func main() {
	cli.Should(startup.RandSeed())

	if err := cmd.Execute(); err != nil {
		if openAIErr := checkOpenAI(err); openAIErr != nil {
			fmt.Fprintf(os.Stderr, "failed to check OpenAI: %v", openAIErr) //nolint:errcheck
		}

		os.Exit(1)
	}
}

//nolint:gocyclo
func checkOpenAI(inputErr error) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	var err error

	apiKey := os.Getenv("TALOSCTL_OPENAI_API_KEY")
	if apiKey == "" {
		return nil
	}

	model := os.Getenv("TALOSCTL_OPENAI_MODEL")
	if model == "" {
		model = openai.GPT3TextDavinci003
	}

	maxTokens := 512

	maxTokensStr := os.Getenv(openAIMaxTokensEnvVar)
	if maxTokensStr != "" {
		maxTokens, err = strconv.Atoi(maxTokensStr)
		if err != nil {
			return fmt.Errorf("failed to parse %s: %w", openAIMaxTokensEnvVar, err)
		}
	}

	talosconfig, talosConfigReadErr := readTalosconfig()

	input := openaiInput{
		Args:               os.Args,
		Err:                inputErr.Error(),
		Talosconfig:        talosconfig,
		TalosconfigReadErr: talosConfigReadErr,
	}

	inputJSON, err := json.Marshal(input)
	if err != nil {
		return fmt.Errorf("failed to marshal input: %w", err)
	}

	client := openai.NewClient(apiKey)

	reportCtx, reportCtxCancel := context.WithCancel(ctx)
	defer reportCtxCancel()

	var wg sync.WaitGroup

	wg.Add(1)

	rep := reporter.New()
	inputStr := string(inputJSON)

	go func() {
		defer wg.Done()

		ticker := time.NewTicker(100 * time.Millisecond)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				rep.Report(reporter.Update{
					Message: fmt.Sprintf("Prompting OPENAI model %q for the error with input: %s", model, inputStr),
					Status:  reporter.StatusRunning,
				})
			case <-reportCtx.Done():
				return
			}
		}
	}()

	result, insufficientTokens, err := completion(ctx, client, model, maxTokens, inputJSON)
	if err != nil {
		if !errors.Is(err, openai.ErrCompletionUnsupportedModel) {
			rep.Report(reporter.Update{
				Message: "OpenAPI prompt failed...",
				Status:  reporter.StatusError,
			})

			return err
		}

		result, insufficientTokens, err = chatCompletion(ctx, client, model, maxTokens, inputJSON)
		if err != nil {
			rep.Report(reporter.Update{
				Message: "OpenAPI prompt failed...",
				Status:  reporter.StatusError,
			})

			return err
		}
	}

	reportCtxCancel()
	wg.Wait()

	rep.Report(reporter.Update{
		Message: fmt.Sprintf("OpenAPI prompt succeeded for input %s.\nSuggestion of model %q (WARNING: USE IT AT YOUR OWN RISK):", inputStr, model),
		Status:  reporter.StatusSucceeded,
	})

	fmt.Fprint(os.Stderr, "\n")   //nolint:errcheck
	fmt.Fprint(os.Stderr, result) //nolint:errcheck
	fmt.Fprint(os.Stderr, "\n")   //nolint:errcheck

	if insufficientTokens {
		fmt.Fprintf(os.Stderr, "<result is cut in half due to insufficient OPENAI API tokens (%d). Try increasing %s>\n", maxTokens, openAIMaxTokensEnvVar) //nolint:errcheck
	}

	return nil
}

func completion(ctx context.Context, client *openai.Client, model string, maxTokens int, inputJSON []byte) (string, bool, error) {
	prompt := fmt.Sprintf("I ran talosctl and got an error. "+
		"Help me to troubleshoot & fix it. Print steps. Be brief. Here's the info: %s\n", string(inputJSON))

	req := openai.CompletionRequest{
		Model:     model,
		Prompt:    prompt,
		MaxTokens: maxTokens,
	}

	resp, err := client.CreateCompletion(ctx, req)
	if err != nil {
		return "", false, fmt.Errorf("failed to create OpenAI completion: %w", err)
	}

	insufficientTokens := false

	choices := slices.Map(resp.Choices, func(t openai.CompletionChoice) string {
		if t.FinishReason == "length" {
			insufficientTokens = true
		}

		return t.Text
	})

	if insufficientTokens {
		choices = append(choices, fmt.Sprintf("<result is cut in half due to insufficient OPENAI API tokens. Try increasing %s (current: %d)>", openAIMaxTokensEnvVar, maxTokens))
	}

	return strings.TrimSpace(strings.Join(choices, "\n")), insufficientTokens, nil
}

func chatCompletion(ctx context.Context, client *openai.Client, model string, maxTokens int, inputJSON []byte) (string, bool, error) {
	req := openai.ChatCompletionRequest{
		Model: model,
		Messages: []openai.ChatCompletionMessage{
			{
				Role:    "system",
				Content: "You are a support agent helping Talos Linux users to help debug & fix their talosctl related issues. You reply with brief info in steps.",
			},
			{
				Role:    "user",
				Content: fmt.Sprintf("Here's the info I gathered as JSON: %s\n", string(inputJSON)),
			},
		},
		MaxTokens: maxTokens,
	}

	resp, err := client.CreateChatCompletion(ctx, req)
	if err != nil {
		return "", false, fmt.Errorf("failed to create OpenAI completion: %w", err)
	}

	insufficientTokens := false

	choices := slices.Map(resp.Choices, func(t openai.ChatCompletionChoice) string {
		if t.FinishReason == "length" {
			insufficientTokens = true
		}

		return t.Message.Content
	})

	return strings.TrimSpace(strings.Join(choices, "\n")), insufficientTokens, nil
}

func readTalosconfig() (map[string]any, error) {
	talosConfig, err := config.Open("")
	if err != nil {
		return nil, err
	}

	for _, configContext := range talosConfig.Contexts {
		configContext.CA = ""
		configContext.Crt = ""
		configContext.Key = ""
	}

	talosConfigBytes, err := talosConfig.Bytes()
	if err != nil {
		return nil, err
	}

	var talosconfigMap map[string]any

	if err = yaml.Unmarshal(talosConfigBytes, &talosconfigMap); err != nil {
		return nil, err
	}

	return talosconfigMap, nil
}
