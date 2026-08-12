import { readdirSync, readFileSync } from 'node:fs'
import { extname, join, relative, resolve } from 'node:path'
import { describe, expect, it } from 'vitest'

const componentsRoot = resolve(process.cwd(), 'src/components')
const allowedInlineColorFiles = new Set<string>()

function componentFiles(directory: string): string[] {
  return readdirSync(directory, { withFileTypes: true }).flatMap((entry) => {
    const path = join(directory, entry.name)
    if (entry.isDirectory()) return componentFiles(path)
    return ['.ts', '.tsx'].includes(extname(entry.name)) ? [path] : []
  })
}

describe('统一设计系统静态约束', () => {
  it('React 组件不直接写十六进制颜色', () => {
    const violations = componentFiles(componentsRoot).flatMap((path) => {
      const source = readFileSync(path, 'utf8')
      const file = relative(componentsRoot, path)
      if (allowedInlineColorFiles.has(file) || !/#(?:[0-9a-f]{3}|[0-9a-f]{6}|[0-9a-f]{8})(?![0-9a-f])/i.test(source)) return []
      return [file]
    })

    expect(violations).toEqual([])
  })

  it('React 组件不创建任意圆角或任意阴影', () => {
    const violations = componentFiles(componentsRoot).flatMap((path) => {
      const source = readFileSync(path, 'utf8')
      return /(?:rounded|shadow)-\[[^\]]+\]/.test(source) ? [relative(componentsRoot, path)] : []
    })

    expect(violations).toEqual([])
  })

  it('统一一级导航不使用蓝色表达品牌或激活态', () => {
    const shell = readFileSync(join(componentsRoot, 'layout', 'WorkspaceShell.tsx'), 'utf8')
    expect(shell).not.toMatch(/(?:bg|border|text|ring)-blue-/)
  })
})
