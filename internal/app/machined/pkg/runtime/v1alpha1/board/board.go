// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package board

import (
	"errors"
	"fmt"
	"os"

	"github.com/talos-systems/go-procfs/procfs"

	"github.com/talos-systems/talos/internal/app/machined/pkg/runtime"
	bananapim64 "github.com/talos-systems/talos/internal/app/machined/pkg/runtime/v1alpha1/board/bananapi_m64"
	jetsonnano "github.com/talos-systems/talos/internal/app/machined/pkg/runtime/v1alpha1/board/jetson_nano"
	libretechallh3cch5 "github.com/talos-systems/talos/internal/app/machined/pkg/runtime/v1alpha1/board/libretech_all_h3_cc_h5"
	"github.com/talos-systems/talos/internal/app/machined/pkg/runtime/v1alpha1/board/pine64"
	"github.com/talos-systems/talos/internal/app/machined/pkg/runtime/v1alpha1/board/rock64"
	rockpi4 "github.com/talos-systems/talos/internal/app/machined/pkg/runtime/v1alpha1/board/rockpi4"
	rpi4 "github.com/talos-systems/talos/internal/app/machined/pkg/runtime/v1alpha1/board/rpi_4"
	"github.com/talos-systems/talos/pkg/machinery/constants"
)

// CurrentBoard is a helper func for discovering the current board.
func CurrentBoard() (b runtime.Board, err error) {
	var board string

	if p := procfs.ProcCmdline().Get(constants.KernelParamBoard).First(); p != nil {
		board = *p
	}

	if p, ok := os.LookupEnv("BOARD"); ok {
		board = p
	}

	if board == "" {
		return nil, errors.New("failed to determine board")
	}

	return newBoard(board)
}

// NewBoard initializes and returns a runtime.Board.
func NewBoard(board string) (b runtime.Board, err error) {
	return newBoard(board)
}

func newBoard(board string) (b runtime.Board, err error) {
	switch board {
	case constants.BoardLibretechAllH3CCH5:
		b = &libretechallh3cch5.LibretechAllH3CCH5{}
	case constants.BoardRPi4:
		b = &rpi4.RPi4{}
	case constants.BoardBananaPiM64:
		b = &bananapim64.BananaPiM64{}
	case constants.BoardPine64:
		b = &pine64.Pine64{}
	case constants.BoardRock64:
		b = &rock64.Rock64{}
	case constants.BoardRockpi4:
		b = &rockpi4.Rockpi4{}
	case constants.BoardJetsonNano:
		b = &jetsonnano.JetsonNano{}
	default:
		return nil, fmt.Errorf("unsupported board: %q", board)
	}

	return b, nil
}
