package testutil

import (
	"fmt"

	"github.com/google/uuid"
)

// RedisAddr is the local Redis every real-infra queue test connects to —
// same "local, host-native service the test suite expects to already be
// running" convention as ConnectDB's Postgres. `brew install redis && brew
// services start redis` is the one-time setup.
const RedisAddr = "localhost:6379"

// UniqueQueueName returns a per-call queue name so concurrent test
// processes (go test runs each package as its own process, often in
// parallel) never share an asynq queue and interfere with each other's
// counts, or with a real marrow serve instance pointed at the same Redis.
func UniqueQueueName(prefix string) string {
	return fmt.Sprintf("%s-test-%s", prefix, uuid.NewString())
}
