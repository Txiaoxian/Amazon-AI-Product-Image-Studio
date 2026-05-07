export interface ImageDimensions {
  width: number
  height: number
}

export async function getImageDimensions(blob: Blob): Promise<ImageDimensions> {
  const url = URL.createObjectURL(blob)

  try {
    return await new Promise<ImageDimensions>((resolve, reject) => {
      const image = new Image()
      image.onload = () => {
        resolve({ width: image.naturalWidth, height: image.naturalHeight })
      }
      image.onerror = () => reject(new Error('无法读取图片尺寸。'))
      image.src = url
    })
  } finally {
    URL.revokeObjectURL(url)
  }
}

export function fileNameTimestamp(date = new Date()): string {
  return date.toISOString().replaceAll(':', '-').replace(/\.\d{3}Z$/, 'Z')
}
