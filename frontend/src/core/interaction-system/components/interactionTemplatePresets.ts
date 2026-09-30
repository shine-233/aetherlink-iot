/**
 * 文件用途：交互模板选择器的预设模板数据。
 * 核心逻辑：按「基础 / 视觉 / 动画 / 复杂」分类提供开箱即用的交互配置。
 * 关键注意事项：模板 id 是用户收藏与去重的依据，不要随意改名；新增模板需同步分类与文案。
 */
import type { Component } from 'vue'
import {
  EyeOutline,
  FlashOutline,
  ColorPaletteOutline,
  PlayOutline,
  SettingsOutline,
  HeartOutline,
  StarOutline
} from '@vicons/ionicons5'

import type { InteractionConfig } from './interactionPreviewTypes'
import type { InteractionTemplate } from './interactionTemplateTypes'

export type InteractionTemplateTranslate = (key: string, named?: Record<string, unknown>) => string

export function getPredefinedTemplates(t: InteractionTemplateTranslate): InteractionTemplate[] {
  return [
    // 基础交互模板
    {
      id: 'click-highlight',
      name: t('interaction.template.predefined.clickHighlight'),
      description: t('interaction.template.predefined.clickHighlightDesc'),
      category: 'basic',
      icon: FlashOutline,
      color: '#18a058',
      config: [
        {
          event: 'click',
          responses: [
            {
              action: 'changeBackgroundColor',
              value: '#ffeb3b',
              duration: 200
            }
          ],
          enabled: true,
          priority: 1,
          name: t('interaction.template.predefined.clickHighlightEffect')
        }
      ]
    },
    {
      id: 'hover-scale',
      name: t('interaction.template.predefined.hoverScale'),
      description: t('interaction.template.predefined.hoverScaleDesc'),
      category: 'basic',
      icon: SettingsOutline,
      color: '#2080f0',
      config: [
        {
          event: 'hover',
          responses: [
            {
              action: 'changeTransform',
              value: 'scale(1.05)',
              duration: 300,
              easing: 'ease-out'
            }
          ],
          enabled: true,
          priority: 1,
          name: t('interaction.template.predefined.hoverScaleEffect')
        }
      ]
    },

    // 视觉效果模板
    {
      id: 'rainbow-border',
      name: t('interaction.template.predefined.rainbowBorder'),
      description: t('interaction.template.predefined.rainbowBorderDesc'),
      category: 'visual',
      icon: ColorPaletteOutline,
      color: '#f0a020',
      config: [
        {
          event: 'click',
          responses: [
            {
              action: 'changeBorderColor',
              value: '#ff4757',
              duration: 200
            },
            {
              action: 'changeBorderColor',
              value: '#3742fa',
              duration: 200,
              delay: 200
            },
            {
              action: 'changeBorderColor',
              value: '#2ed573',
              duration: 200,
              delay: 400
            }
          ],
          enabled: true,
          priority: 1,
          name: t('interaction.template.predefined.rainbowBorderEffect')
        }
      ]
    },
    {
      id: 'fade-toggle',
      name: t('interaction.template.predefined.fadeToggle'),
      description: t('interaction.template.predefined.fadeToggleDesc'),
      category: 'visual',
      icon: EyeOutline,
      color: '#7c3aed',
      config: [
        {
          event: 'click',
          responses: [
            {
              action: 'changeOpacity',
              value: 0.3,
              duration: 500,
              easing: 'ease-in-out'
            }
          ],
          enabled: true,
          priority: 1,
          name: t('interaction.template.predefined.transparencyToggle')
        }
      ]
    },

    // 动画效果模板
    {
      id: 'pulse-animation',
      name: t('interaction.template.predefined.pulseAnimation'),
      description: t('interaction.template.predefined.pulseAnimationDesc'),
      category: 'animation',
      icon: HeartOutline,
      color: '#e74c3c',
      config: [
        {
          event: 'click',
          responses: [
            {
              action: 'triggerAnimation',
              value: 'pulse',
              duration: 1000,
              easing: 'ease-in-out'
            }
          ],
          enabled: true,
          priority: 1,
          name: t('interaction.template.predefined.pulseAnimationName')
        }
      ]
    },
    {
      id: 'shake-animation',
      name: t('interaction.template.predefined.shakeAnimation'),
      description: t('interaction.template.predefined.shakeAnimationDesc'),
      category: 'animation',
      icon: PlayOutline,
      color: '#f39c12',
      config: [
        {
          event: 'blur',
          responses: [
            {
              action: 'triggerAnimation',
              value: 'shake',
              duration: 600,
              easing: 'ease-in-out'
            }
          ],
          enabled: true,
          priority: 1,
          name: t('interaction.template.predefined.shakeTip')
        }
      ]
    },

    // 复合交互模板
    {
      id: 'complete-feedback',
      name: t('interaction.template.predefined.completeFeedback'),
      description: t('interaction.template.predefined.completeFeedbackDesc'),
      category: 'complex',
      icon: StarOutline,
      color: '#9b59b6',
      config: [
        {
          event: 'hover',
          responses: [
            {
              action: 'changeBackgroundColor',
              value: '#f8f9fa',
              duration: 200
            }
          ],
          enabled: true,
          priority: 3,
          name: t('interaction.template.predefined.hoverFeedback')
        },
        {
          event: 'click',
          responses: [
            {
              action: 'changeBackgroundColor',
              value: '#007bff',
              duration: 100
            },
            {
              action: 'changeTextColor',
              value: '#ffffff',
              duration: 100
            }
          ],
          enabled: true,
          priority: 2,
          name: t('interaction.template.predefined.clickFeedback')
        },
        {
          event: 'focus',
          responses: [
            {
              action: 'changeBorderColor',
              value: '#007bff',
              duration: 200
            }
          ],
          enabled: true,
          priority: 1,
          name: t('interaction.template.predefined.focusFeedback')
        }
      ]
    }
  ]
}
