export interface SolutionEditorSyncState {
  markdown: string;
  baselineHash: string;
  dirty: boolean;
  remoteUpdateAvailable: boolean;
}

export interface SolutionWorkingSnapshot {
  markdown: string;
  content_hash: string;
}

export function reconcileSolutionEditor(
  state: SolutionEditorSyncState,
  remote: SolutionWorkingSnapshot | null | undefined
): SolutionEditorSyncState {
  if (!remote) return state;
  if (state.dirty && state.baselineHash && remote.content_hash !== state.baselineHash) {
    return { ...state, remoteUpdateAvailable: true };
  }
  if (state.dirty) return state;
  return {
    markdown: remote.markdown || '',
    baselineHash: remote.content_hash || '',
    dirty: false,
    remoteUpdateAvailable: false
  };
}
