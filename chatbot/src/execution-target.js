export function executionLocked(session) {
  return !!(session?.executionTargetLocked || session?.messages?.length ||
    session?.runId || session?.workspaceRunId || session?.ownWorkspaceRunIds?.length ||
    session?.parentSessionId || session?.taskId);
}

export function retiredCloudSession(session) {
  return executionLocked(session) && session?.executionTarget?.pool === "cloud" &&
    !session.runId && !session.taskId && session.source !== "scheduled";
}
