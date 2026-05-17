import { ImagePlus, X } from 'lucide-react'
import { useRef, useState } from 'react'
import { validateImageFile } from '../../lib/file'
import type { WorkbenchReferenceInput } from '../../types/workbench'

interface ImageDropzoneProps {
  references: WorkbenchReferenceInput[]
  maxReferences?: number
  onChange: (references: WorkbenchReferenceInput[]) => void
  onError: (message: string) => void
  disabled?: boolean
}

export function ImageDropzone({ references, maxReferences, onChange, onError, disabled }: ImageDropzoneProps) {
  const inputRef = useRef<HTMLInputElement>(null)
  const [isDragging, setIsDragging] = useState(false)

  const addFiles = (files: FileList | File[]) => {
    const nextReferences: WorkbenchReferenceInput[] = []

    for (const file of Array.from(files)) {
      try {
        validateImageFile(file)
        nextReferences.push({
          kind: 'pending',
          file,
          previewUrl: URL.createObjectURL(file),
        })
      } catch (error) {
        onError(error instanceof Error ? error.message : '参考图无效。')
      }
    }

    if (nextReferences.length === 0) {
      return
    }

    const availableSlots = maxReferences === undefined ? nextReferences.length : Math.max(maxReferences - references.length, 0)
    if (availableSlots === 0) {
      nextReferences.forEach((reference) => URL.revokeObjectURL(reference.previewUrl))
      onError(maxReferences === 1 ? '当前模型仅支持 1 张参考图。' : `当前模型最多支持 ${maxReferences} 张参考图。`)
      return
    }

    if (nextReferences.length > availableSlots) {
      nextReferences.slice(availableSlots).forEach((reference) => URL.revokeObjectURL(reference.previewUrl))
      onError(maxReferences === 1 ? '当前模型仅支持 1 张参考图。' : `当前模型最多支持 ${maxReferences} 张参考图。`)
    }

    onChange([...references, ...nextReferences.slice(0, availableSlots)])
  }

  const removeReference = (previewUrl: string) => {
    URL.revokeObjectURL(previewUrl)
    onChange(references.filter((reference) => reference.previewUrl !== previewUrl))
  }

  return (
    <section className="space-y-3">
      <div className="flex items-center justify-between">
        <label className="field-label" htmlFor="reference-images">
          参考图
        </label>
        <span className="text-xs text-ink-400">{references.length} 张</span>
      </div>

      <button
        className={`flex min-h-28 w-full flex-col items-center justify-center gap-2 rounded-lg border border-dashed px-4 py-5 text-center transition ${
          isDragging ? 'border-amazon-500 bg-amazon-500/10' : 'border-ink-300 bg-ink-50 hover:bg-white'
        }`}
        disabled={disabled}
        onClick={() => inputRef.current?.click()}
        onDragLeave={() => setIsDragging(false)}
        onDragOver={(event) => {
          event.preventDefault()
          setIsDragging(true)
        }}
        onDrop={(event) => {
          event.preventDefault()
          setIsDragging(false)
          addFiles(event.dataTransfer.files)
        }}
        type="button"
      >
        <ImagePlus className="h-6 w-6 text-ink-500" />
        <span className="text-sm font-medium text-ink-700">上传或拖入图片</span>
        <span className="text-xs text-ink-400">JPG / PNG / WebP，单张 15MB 内</span>
      </button>

      <input
        ref={inputRef}
        accept="image/jpeg,image/png,image/webp"
        className="hidden"
        disabled={disabled}
        id="reference-images"
        multiple
        onChange={(event) => {
          if (event.target.files) {
            addFiles(event.target.files)
          }
          event.target.value = ''
        }}
        type="file"
      />

      {references.length > 0 ? (
        <div className="grid grid-cols-3 gap-2">
          {references.map((reference) => (
            <div
              className="group relative aspect-square overflow-hidden rounded-md border border-ink-200 bg-ink-100"
              key={reference.kind === 'asset' ? reference.assetId : reference.previewUrl}
            >
              <img alt={getReferenceFilename(reference)} className="h-full w-full object-cover" src={reference.previewUrl} />
              <button
                aria-label={`删除 ${getReferenceFilename(reference)}`}
                className="absolute right-1 top-1 hidden h-7 w-7 items-center justify-center rounded-md bg-white/90 text-ink-700 shadow group-hover:flex"
                disabled={disabled}
                onClick={() => removeReference(reference.previewUrl)}
                type="button"
              >
                <X className="h-4 w-4" />
              </button>
            </div>
          ))}
        </div>
      ) : null}
    </section>
  )
}

function getReferenceFilename(reference: WorkbenchReferenceInput): string {
  return reference.kind === 'asset' ? reference.filename : reference.file.name
}
