import { describe, expect, it } from 'vitest'
import zh from '../zh'
import en from '../en'

describe('plugin locale copy', () => {
  it('does not use invalid vue-i18n placeholders in plugin descriptions', () => {
    expect(zh.home).toBeTruthy()
    expect(zh.admin.settings.plugins.description).not.toContain('{插件名}')
    expect(zh.admin.settings.plugins.hints.directoryRule).not.toContain('{插件名}')
    expect(en.admin.settings.plugins.description).not.toContain('{plugin-name}')
    expect(en.admin.settings.plugins.hints.directoryRule).not.toContain('{plugin-name}')
  })
})
