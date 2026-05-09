export interface ImageResizeDimensions {
  width: number
  height: number
}

export async function resizeImageBlob(blob: Blob, dimensions: ImageResizeDimensions): Promise<Blob> {
  const image = await loadImage(blob)
  const canvas = document.createElement('canvas')
  canvas.width = dimensions.width
  canvas.height = dimensions.height

  const context = canvas.getContext('2d')

  if (!context) {
    throw new Error('当前浏览器不支持图片缩放。')
  }

  context.imageSmoothingEnabled = true
  context.imageSmoothingQuality = 'high'
  context.drawImage(image, 0, 0, dimensions.width, dimensions.height)

  return new Promise((resolve, reject) => {
    canvas.toBlob(
      (resizedBlob) => {
        if (!resizedBlob) {
          reject(new Error('图片缩放失败，请重试。'))
          return
        }

        resolve(resizedBlob)
      },
      blob.type || 'image/png',
    )
  })
}

function loadImage(blob: Blob): Promise<HTMLImageElement> {
  return new Promise((resolve, reject) => {
    const url = URL.createObjectURL(blob)
    const image = new Image()

    image.onload = () => {
      URL.revokeObjectURL(url)
      resolve(image)
    }
    image.onerror = () => {
      URL.revokeObjectURL(url)
      reject(new Error('无法读取需要缩放的图片。'))
    }
    image.src = url
  })
}
