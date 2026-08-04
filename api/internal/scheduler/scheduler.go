package scheduler

import (
	"context"
	"log"

	"github.com/robfig/cron/v3"
)

// Task is one scheduled unit of work. Cron is a standard cron expression or
// a robfig/cron descriptor (e.g. "@every 1m", "@hourly").
type Task interface {
	Name() string
	Cron() string
	Run(ctx context.Context) error
}

// Scheduler is a single shared cron runner. Contexts register what/when to
// run via Schedule(task) — there is no per-context Scheduler wrapper.
type Scheduler struct {
	cron *cron.Cron
}

func New() *Scheduler {
	return &Scheduler{cron: cron.New()}
}

// Schedule registers task to run on its own Cron expression.
func (s *Scheduler) Schedule(task Task) error {
	_, err := s.cron.AddFunc(task.Cron(), func() {
		if err := task.Run(context.Background()); err != nil {
			log.Printf("scheduled task %q failed: %v", task.Name(), err)
		}
	})
	return err
}

func (s *Scheduler) Start() {
	s.cron.Start()
}

// Stop halts the scheduler and blocks until any currently running task
// finishes.
func (s *Scheduler) Stop() {
	<-s.cron.Stop().Done()
}
