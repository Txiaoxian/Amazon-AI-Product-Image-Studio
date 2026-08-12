import { readFileSync } from 'node:fs'
import { resolve } from 'node:path'
import { describe, expect, it } from 'vitest'
import { APP_NAME } from '../lib/constants'

describe('产品品牌标识', () => {
  it('浏览器标签和应用名称统一使用简体中文', () => {
    const indexHtml = readFileSync(resolve(process.cwd(), 'index.html'), 'utf8')

    expect(APP_NAME).toBe('亚马逊 AI 商品图工作室')
    expect(indexHtml).toContain(`<title>${APP_NAME}</title>`)
    expect(indexHtml).toContain('href="/favicon-32.png?v=3"')
    expect(indexHtml).toContain('href="/favicon-16.png?v=3"')
    expect(indexHtml).toContain('href="/apple-touch-icon.png?v=3"')
    expect(indexHtml).not.toContain('favicon.svg')
  })

  it.each([
    ['favicon-16.png', 16],
    ['favicon-32.png', 32],
    ['favicon-64.png', 64],
    ['apple-touch-icon.png', 180],
  ])('%s 使用选定的 AI 生图图标并提供正确尺寸', (filename, size) => {
    const favicon = readFileSync(resolve(process.cwd(), `public/${filename}`))

    expect(favicon.subarray(0, 8)).toEqual(Buffer.from([137, 80, 78, 71, 13, 10, 26, 10]))
    expect(favicon.readUInt32BE(16)).toBe(size)
    expect(favicon.readUInt32BE(20)).toBe(size)
  })
})
