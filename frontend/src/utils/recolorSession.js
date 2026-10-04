const sessions = new Map()

// Uploads are identified by their working-copy id, so re-uploading a file with
// the same name starts a clean session while resuming an old one restores it.
function sourceKey(source) {
  if (source.kind === 'upload') return 'upload:' + (source.sessionId || source.packName || '')
  return (source.kind || '') + ':' + (source.dirName || '')
}

export function loadSession(source) {
  return sessions.get(sourceKey(source)) || null
}

export function saveSession(source, session) {
  const key = sourceKey(source)
  if (!session) sessions.delete(key)
  else sessions.set(key, session)
}

export function clearSession(source) {
  sessions.delete(sourceKey(source))
}