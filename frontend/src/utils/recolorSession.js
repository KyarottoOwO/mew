const sessions = new Map()

function sourceKey(source) {
  return (source.kind || '') + ':' + (source.dirName || source.packName || '')
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