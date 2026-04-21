package daemon

func (d *Daemon) rememberTaskToken(taskID, token string) {
	if taskID == "" || token == "" {
		return
	}
	d.mu.Lock()
	d.taskTokens[taskID] = token
	d.mu.Unlock()
}

func (d *Daemon) lookupTaskToken(taskID string) string {
	if taskID == "" {
		return ""
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.taskTokens[taskID]
}

func (d *Daemon) clearTaskToken(taskID string) {
	if taskID == "" {
		return
	}
	d.mu.Lock()
	delete(d.taskTokens, taskID)
	d.mu.Unlock()
}
