package daemon

func (d *Daemon) rememberTaskBranch(taskID, branchName string) {
	if taskID == "" || branchName == "" {
		return
	}
	d.mu.Lock()
	d.taskBranches[taskID] = branchName
	d.mu.Unlock()
}

func (d *Daemon) consumeTaskBranch(taskID string) string {
	if taskID == "" {
		return ""
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	branchName := d.taskBranches[taskID]
	delete(d.taskBranches, taskID)
	return branchName
}

func (d *Daemon) clearTaskBranch(taskID string) {
	if taskID == "" {
		return
	}
	d.mu.Lock()
	delete(d.taskBranches, taskID)
	d.mu.Unlock()
}
