package boot

import (
	"github.com/talos-systems/talos/internal/app/machined/internal/phase"
)

type Status int

const (
	Pass Status = iota
	Fail
)

type Level int

const (
	Zero Level = iota
	One
	Two
	Three
	Four
	Five
	Six
	Seven
	Eight
	Nine
)

const (
	MaxLevel = Nine
)

type LevelRunner struct {
	status Status
	phases []phase.Phase
}

type Levels [][]phase.Phase

func (l Levels) Append(p phase.Phase, level Level) {
	l[level] = append(l[level], p)
}

type BootSequencer struct {
	*phase.Runner

	levels Levels
}

func (b *BootSequencer) Register(p phase.Phase, l Level) {
	b.levels[l] = append(b.levels[l], p)
}

var boot *BootSequencer

func init() {
	phaserunner, err := phase.NewRunner(nil)
	if err != nil {
		panic(err)
	}
	boot = &BootSequencer{
		Runner: phaserunner,
		levels: make(Levels, 0, MaxLevel),
	}

	for i := 0; i < int(MaxLevel); i++ {
		boot.levels = append(boot.levels, make([]phase.Phase, 0))
	}
}
