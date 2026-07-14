import { Check, ImagePlus, X } from 'lucide-react'
import { useRef, useState } from 'react'
import { validateImageFile } from '../../lib/file'
import type { WorkbenchReferenceInput } from '../../types/workbench'

interface ImageDropzoneProps {
  availableReferences?: WorkbenchReferenceInput[]
  references: WorkbenchReferenceInput[]
  maxReferences?: number
  onChange: (references: WorkbenchReferenceInput[]) => void
  onError: (message: string) => void
  disabled?: boolean
  allowUpload?: boolean
}

export function ImageDropzone({
  availableReferences = [],
  references,
  maxReferences,
  onChange,
  onError,
  disabled,
  allowUpload = true,
}: ImageDropzoneProps) {
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
    const removedReference = references.find((reference) => reference.previewUrl === previewUrl)
    if (removedReference?.kind === 'pending') {
      URL.revokeObjectURL(removedReference.previewUrl)
    }
    onChange(references.filter((reference) => reference.previewUrl !== previewUrl))
  }

  const toggleAvailableReference = (candidate: WorkbenchReferenceInput) => {
    if (candidate.kind !== 'asset') {
      return
    }

    const selected = references.some((reference) => reference.kind === 'asset' && reference.assetId === candidate.assetId)
    if (selected) {
      onChange(references.filter((reference) => reference.kind !== 'asset' || reference.assetId !== candidate.assetId))
      return
    }

    if (maxReferences !== undefined && references.length >= maxReferences) {
      onError(maxReferences === 1 ? '当前模型仅支持 1 张参考图。' : `当前模型最多支持 ${maxReferences} 张参考图。`)
      return
    }

    onChange([...references, candidate])
  }

  return (
    <section className="space-y-3">
      <div className="flex items-center justify-between">
        <label className="field-label" htmlFor="reference-images">
          参考图（可选）
        </label>
        <span className="text-xs text-ink-400">{references.length} 张</span>
      </div>

      {availableReferences.length > 0 ? (
        <div>
          <p className="mb-2 text-xs font-medium text-ink-500">从产品素材选择</p>
          <div className="flex gap-2 overflow-x-auto pb-1">
            {availableReferences.map((reference) => {
              const filename = getReferenceFilename(reference)
              const selected = reference.kind === 'asset' && references.some(
                (candidate) => candidate.kind === 'asset' && candidate.assetId === reference.assetId,
              )
              return (
                <button
                  aria-label={`${selected ? '取消选择' : '选择'}产品参考图 ${filename}`}
                  aria-pressed={selected}
                  className={`group relative h-16 w-16 shrink-0 overflow-hidden rounded-md border-2 transition focus:outline-none focus:ring-2 focus:ring-amazon-500/30 ${
                    selected ? 'border-amazon-500' : 'border-ink-200 hover:border-ink-400'
                  }`}
                  disabled={disabled}
                  key={reference.kind === 'asset' ? reference.assetId : reference.previewUrl}
                  onClick={() => toggleAvailableReference(reference)}
                  title={filename}
                  type="button"
                >
                  <img alt="" className="h-full w-full object-cover" src={reference.previewUrl} />
                  {selected ? (
                    <span className="absolute right-1 top-1 flex h-5 w-5 items-center justify-center rounded-full bg-amazon-500 text-ink-950">
                      <Check className="h-3.5 w-3.5" />
                    </span>
                  ) : null}
                </button>
              )
            })}
          </div>
        </div>
      ) : null}

      {allowUpload ? (
        <>
          <button
            className={`flex min-h-20 w-full items-center justify-center gap-3 rounded-lg border border-dashed px-4 py-3 text-left transition ${
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
            <span>
              <span className="block text-sm font-medium text-ink-700">上传新的参考图</span>
              <span className="mt-0.5 block text-xs text-ink-400">JPG / PNG / WebP，单张 15MB 内</span>
            </span>
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
        </>
      ) : (
        <div className="rounded-lg border border-dashed border-ink-300 bg-ink-50 px-4 py-4 text-sm leading-6 text-ink-500">
          请先在产品素材库上传参考图，再点击“作为参考图”加入当前任务。
        </div>
      )}

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
                className="absolute right-1 top-1 flex h-7 w-7 items-center justify-center rounded-md bg-white/90 text-ink-700 opacity-0 shadow transition-opacity group-hover:opacity-100 group-focus-within:opacity-100 focus:opacity-100"
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
