import { defineComponent, h } from 'vue'
import type { ComponentCustomProperties } from 'vue'
import { mount } from '@vue/test-utils'
import { afterEach, describe, expect, it } from 'vitest'

import AddDeviceDrawer from '../add-device-drawer.vue'

const DrawerStub = defineComponent({
  name: 'NDrawer',
  props: ['show', 'placement', 'height'],
  emits: ['update:show', 'afterLeave'],
  setup(props, { emit, slots }) {
    return () =>
      h('div', { 'data-testid': 'drawer', 'data-show': String(props.show) }, [
        h('button', { 'data-testid': 'drawer-close', onClick: () => emit('update:show', false) }, 'close'),
        h('button', { 'data-testid': 'drawer-after-leave', onClick: () => emit('afterLeave') }, 'left'),
        slots.default?.()
      ])
  }
})

const DrawerContentStub = defineComponent({
  name: 'NDrawerContent',
  props: ['title'],
  setup(props, { slots }) {
    return () => h('section', [h('h2', props.title), slots.default?.()])
  }
})

const StepsStub = defineComponent({
  name: 'NSteps',
  props: ['current', 'status'],
  setup(props, { slots }) {
    return () =>
      h(
        'ol',
        {
          'data-testid': 'manual-steps',
          'data-current': String(props.current),
          'data-status': props.status
        },
        slots.default?.()
      )
  }
})

const StepStub = defineComponent({
  name: 'NStep',
  props: ['title', 'description'],
  setup(props) {
    return () => h('li', `${props.title}: ${props.description}`)
  }
})

const ButtonStub = defineComponent({
  name: 'NButton',
  props: { disabled: Boolean },
  emits: ['click'],
  setup(props, { emit, slots }) {
    return () =>
      h(
        'button',
        {
          disabled: props.disabled,
          onClick: () => {
            if (!props.disabled) emit('click')
          }
        },
        slots.default?.()
      )
  }
})

const InputStub = defineComponent({
  name: 'NInput',
  props: ['value', 'placeholder', 'maxlength'],
  emits: ['update:value'],
  setup(props, { emit }) {
    return () =>
      h('input', {
        value: props.value,
        placeholder: props.placeholder,
        maxlength: props.maxlength,
        onInput: (event: Event) => emit('update:value', (event.target as HTMLInputElement).value)
      })
  }
})

const Step1Stub = defineComponent({
  name: 'AddDevicesStep1',
  props: ['setIdCallback', 'configOptions', 'nextCallback'],
  setup(props) {
    return () =>
      h('div', [
        h(
          'button',
          {
            'data-testid': 'step-1-select',
            onClick: () => props.setIdCallback('device-7', 'config-3', '{"name":"Pump"}', 'pump-007')
          },
          'select device'
        ),
        h('button', { 'data-testid': 'step-1-next', onClick: () => props.nextCallback() }, 'next')
      ])
  }
})

const Step2Stub = defineComponent({
  name: 'AddDevicesStep2',
  props: ['setIsSuccess', 'device_id', 'deviceNumber', 'formData', 'formElements', 'nextCallback'],
  setup(props) {
    return () =>
      h('div', [
        h('button', { 'data-testid': 'step-2-success', onClick: () => props.setIsSuccess(true) }, 'success'),
        h('button', { 'data-testid': 'step-2-next', onClick: () => props.nextCallback() }, 'next')
      ])
  }
})

const Step3Stub = defineComponent({
  name: 'AddDevicesStep3',
  props: ['isSuccess', 'device_id', 'device_config_id', 'firstDeviceOnboarding', 'closeCallback', 'backCallback'],
  setup(props) {
    return () =>
      h('div', [
        h('button', { 'data-testid': 'step-3-close', onClick: () => props.closeCallback() }, 'close result'),
        h('button', { 'data-testid': 'step-3-back', onClick: () => props.backCallback() }, 'back')
      ])
  }
})

const SlotStub = defineComponent({
  setup(_, { slots }) {
    return () => h('div', slots.default?.())
  }
})

const baseProps = {
  show: true,
  addKey: 'hands',
  placement: 'right' as const,
  manualStep: 1,
  manualStatus: 'process' as const,
  configOptions: [{ label: 'Pump profile', value: 'config-3' }],
  deviceId: 'device-7',
  deviceConfigId: 'config-3',
  manualDeviceNumber: 'pump-007',
  deviceFormData: { location: 'Line A' },
  formElements: [{ key: 'location' }],
  isSuccess: false,
  deviceNumber: '',
  buttonDisabled: true,
  showMessage: false,
  messageStyle: { color: 'red' },
  firstDeviceOnboarding: true
}

const mountedWrappers: Array<ReturnType<typeof mount>> = []

function mountDrawer(props: Partial<typeof baseProps> = {}) {
  const wrapper = mount(AddDeviceDrawer, {
    props: { ...baseProps, ...props },
    global: {
      config: {
        globalProperties: {
          $t: (key: string) => key
        } as unknown as ComponentCustomProperties
      },
      stubs: {
        NDrawer: DrawerStub,
        NDrawerContent: DrawerContentStub,
        NSteps: StepsStub,
        NStep: StepStub,
        NCard: SlotStub,
        NH4: SlotStub,
        NLi: SlotStub,
        NText: SlotStub,
        NInput: InputStub,
        NButton: ButtonStub,
        AddDevicesStep1: Step1Stub,
        AddDevicesStep2: Step2Stub,
        AddDevicesStep3: Step3Stub
      }
    }
  })
  mountedWrappers.push(wrapper)
  return wrapper
}

function findButton(wrapper: ReturnType<typeof mountDrawer>, text: string) {
  const button = wrapper.findAll('button').find((candidate) => candidate.text() === text)
  if (!button) throw new Error(`Missing button: ${text}`)
  return button
}

describe('add-device-drawer business flow', () => {
  afterEach(() => {
    while (mountedWrappers.length) mountedWrappers.pop()?.unmount()
  })

  it('renders manual onboarding and forwards the selected device before advancing', async () => {
    const wrapper = mountDrawer()

    expect(wrapper.get('h2').text()).toBe('generate.manually-add-device')
    expect(wrapper.get('[data-testid="manual-steps"]').attributes()).toMatchObject({
      'data-current': '1',
      'data-status': 'process'
    })
    expect(wrapper.text()).toContain('custom.devicePage.step1Title: custom.devicePage.step1Desc')
    expect(wrapper.getComponent({ name: 'AddDevicesStep1' }).props('configOptions')).toEqual(baseProps.configOptions)

    await wrapper.get('[data-testid="step-1-select"]').trigger('click')
    await wrapper.get('[data-testid="step-1-next"]').trigger('click')

    expect(wrapper.emitted('setUpId')).toEqual([['device-7', 'config-3', '{"name":"Pump"}', 'pump-007']])
    expect(wrapper.emitted('update:manualStep')).toEqual([[2]])
  })

  it('keeps number completion disabled until valid input and exposes the new value', async () => {
    const wrapper = mountDrawer({
      addKey: 'number',
      deviceNumber: 'bad',
      buttonDisabled: true,
      showMessage: true
    })

    expect(wrapper.get('h2').text()).toBe('custom.devicePage.addByNumber')
    expect(wrapper.text()).toContain('custom.devicePage.deviceNumberNotAvailable')
    expect(wrapper.get('input').attributes()).toMatchObject({
      maxlength: '12',
      placeholder: 'custom.devicePage.enterDeviceNumber'
    })

    await wrapper.get('input').setValue('123456789012')
    await findButton(wrapper, 'custom.devicePage.finish').trigger('click')

    expect(wrapper.emitted('update:deviceNumber')).toEqual([['123456789012']])
    expect(wrapper.emitted('completeNumberAdd')).toBeUndefined()

    await wrapper.setProps({ buttonDisabled: false })
    await findButton(wrapper, 'custom.devicePage.finish').trigger('click')

    expect(wrapper.emitted('completeNumberAdd')).toEqual([[]])
    expect(wrapper.text()).toContain('custom.devicePage.enterDeviceNumber')
  })

  it('propagates success, back, close, and drawer lifecycle actions', async () => {
    const step2 = mountDrawer({ manualStep: 2 })
    await step2.get('[data-testid="step-2-success"]').trigger('click')
    await step2.get('[data-testid="step-2-next"]').trigger('click')

    expect(step2.emitted('setIsSuccess')).toEqual([[true]])
    expect(step2.emitted('update:manualStep')).toEqual([[3]])

    const step3 = mountDrawer({ manualStep: 3, isSuccess: true })
    expect(step3.getComponent({ name: 'AddDevicesStep3' }).props()).toMatchObject({
      isSuccess: true,
      device_id: 'device-7',
      device_config_id: 'config-3',
      firstDeviceOnboarding: true
    })

    await step3.get('[data-testid="step-3-back"]').trigger('click')
    await step3.get('[data-testid="step-3-close"]').trigger('click')
    await step3.get('[data-testid="drawer-after-leave"]').trigger('click')

    expect(step3.emitted('update:manualStep')).toEqual([[2]])
    expect(step3.emitted('update:show')).toEqual([[false]])
    expect(step3.emitted('afterLeave')).toEqual([[]])
  })
})
