import { useState } from 'react'
import { urls } from '../api/client'
import type { MediaFile } from '../api/types'
import { formatBytes, playableAs } from '../lib/format'
import { Dialog } from './Dialog'

interface Props {
  file: MediaFile | null
  title?: string
  onClose: () => void
}

/** Modal in-browser preview using /files/{id}/stream (Range-capable, inline). */
export function PlayerDialog({ file, title, onClose }: Props) {
  const kind = file ? playableAs(file) : null
  return (
    <Dialog
      open={!!file}
      onClose={onClose}
      title={title ?? file?.name ?? 'Preview'}
      description={file ? `${file.name} · ${formatBytes(file.size)}` : undefined}
      size={kind === 'video' ? 'xl' : 'md'}
      bodyClassName="p-0 sm:p-0"
    >
      {file && kind && <PlayerBody key={file.id} file={file} kind={kind} />}
    </Dialog>
  )
}

function PlayerBody({ file, kind }: { file: MediaFile; kind: 'video' | 'audio' }) {
  const [failed, setFailed] = useState(false)
  if (failed) {
    return (
      <p className="px-6 py-10 text-center text-sm text-ink-400">
        Your browser can’t play this file. Download it and open it in a media player instead.
      </p>
    )
  }
  if (kind === 'video') {
    return (
      <video
        src={urls.fileStream(file.id)}
        controls
        autoPlay
        playsInline
        preload="metadata"
        onError={() => setFailed(true)}
        className="aspect-video max-h-[70dvh] w-full bg-black"
      />
    )
  }
  return (
    <div className="p-5 sm:p-6">
      <audio
        src={urls.fileStream(file.id)}
        controls
        autoPlay
        preload="metadata"
        onError={() => setFailed(true)}
        className="w-full"
      />
    </div>
  )
}
