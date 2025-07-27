// SetupTest initializes runtime and state for the test.
func (suite *DefaultSuite) SetupTest() {
	suite.ctx, suite.ctxCancel = context.WithTimeout(context.Background(), 3*time.Minute)

	// Increase file descriptor limits for tests to prevent "too many open files" errors
	var rLimit syscall.Rlimit
	if err := syscall.Getrlimit(syscall.RLIMIT_NOFILE, &rLimit); err == nil {
		originalLimit := rLimit
		rLimit.Cur = 65536
		if rLimit.Max < rLimit.Cur {
			rLimit.Max = rLimit.Cur
		}
		
		if err := syscall.Setrlimit(syscall.RLIMIT_NOFILE, &rLimit); err != nil {
			suite.T().Logf("Warning: Failed to increase file descriptor limit: %v", err)
		}
		
		// Restore original limit after test completes
		suite.T().Cleanup(func() {
			if err := syscall.Setrlimit(syscall.RLIMIT_NOFILE, &originalLimit); err != nil {
				suite.T().Logf("Warning: Failed to restore file descriptor limit: %v", err)
			}
		})
	}

	var err error

	suite.rtState, err = state.NewState()
	suite.Require().NoError(err)

	suite.runtime, err = runtime.NewRuntime(suite.rtState)
	suite.Require().NoError(err)
}
