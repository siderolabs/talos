package controller

import (
	"context"
	"fmt"
	"log"
	"time"
)

const dummySourceInterval = time.Minute

type Controller struct {
	change chan struct{}
}

func (c *Controller) Run(ctx context.Context) error {

	if c.change == nil {
		c.change = make(chan struct{})
	}

	// Until we have COSI, construct a dummy event source
	go func(ctx context.Context) {
		t := time.NewTicker(dummySourceInterval)
		defer t.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
				c.change <- struct{}{}
			}
		}
	}(ctx)

	for {
		select {
		case <-ctx.Done():
			return nil
		case <-c.change:
			changed, err := c.Reconcile(ctx)
			if changed {
				select {
				case c.change <- struct{}{}:
				default:
				}
			}
			if err != nil {
				log.Printf("failed to reconcile: %s", err.Error())
			}
		}
	}
}

func (c *Controller) Reconcile(ctx context.Context) (changed bool, err error) {

	// process interfaces ↦ addresses ↦ routes


	return false, fmt.Errorf("unimplemented")
}
