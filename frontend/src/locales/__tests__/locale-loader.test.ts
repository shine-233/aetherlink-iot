import { describe, expect, it } from 'vitest'
import { getLangMessages } from '../locale'

describe('locale message loader', () => {
  it('merges compatibility-flat files and namespaces feature files', () => {
    const modules = {
      './langs/en-us/common.json': {
        default: {
          common: {
            confirm: 'Confirm'
          }
        }
      },
      './langs/en-us/buttons.json': {
        default: {
          buttons: {
            save: 'Save'
          }
        }
      },
      './langs/en-us/rdi.json': {
        default: {
          overview: {
            title: 'RDI overview'
          }
        }
      },
      './langs/zh-cn/common.json': {
        default: {
          common: {
            confirm: '确认'
          }
        }
      }
    }

    expect(getLangMessages(modules, 'en-us')).toEqual({
      common: {
        confirm: 'Confirm'
      },
      buttons: {
        save: 'Save'
      },
      rdi: {
        overview: {
          title: 'RDI overview'
        }
      }
    })
  })

  it('ignores modules outside the requested language folder', () => {
    const modules = {
      './langs/en-us/rdi.json': { default: { title: 'English' } },
      './langs/fr-fr/rdi.json': { default: { title: 'Français' } },
      './unrelated/en-us/rdi.json': { default: { title: 'Unrelated' } }
    }

    expect(getLangMessages(modules, 'fr-fr')).toEqual({
      rdi: {
        title: 'Français'
      }
    })
  })
})
