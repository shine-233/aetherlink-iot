/**
 * 文件用途：封装 Vue provide/inject 的上下文传递模式。
 * 核心逻辑：通过 InjectionKey 提供泛型安全的 provider 和 consumer 创建函数。
 * 关键注意事项：consumer 必须在 provider 后代组件中调用，否则会返回默认值或 undefined。
 * 重构建议：可增加必填上下文的错误提示封装，提升遗漏 provider 时的可诊断性。
 */
import { inject, provide } from 'vue'
import type { InjectionKey } from 'vue'

/**
 * Use context
 *
 * @example
 *   ```ts
 *   // there are three vue files: A.vue, B.vue, C.vue, and A.vue is the parent component of B.vue and C.vue
 *
 *   // context.ts
 *   import { ref } from 'vue';
 *   import { useContext } from '@aetherlink/hooks';
 *
 *   export const { setupStore, useStore } = useContext('demo', () => {
 *     const count = ref(0);
 *
 *     function increment() {
 *       count.value++;
 *     }
 *
 *     function decrement() {
 *       count.value--;
 *     }
 *
 *     return {
 *       count,
 *       increment,
 *       decrement
 *     };
 *   })
 *   ``` // A.vue
 *   ```vue
 *   <template>
 *     <div>A</div>
 *   </template>
 *   <script setup lang="ts">
 *   import { setupStore } from './context';
 *
 *   setupStore();
 *   // const { increment } = setupStore(); // also can control the store in the parent component
 *   </script>
 *   ``` // B.vue
 *   ```vue
 *   <template>
 *    <div>B</div>
 *   </template>
 *   <script setup lang="ts">
 *   import { useStore } from './context';
 *
 *   const { count, increment } = useStore();
 *   </script>
 *   ```;
 *
 *   // C.vue is same as B.vue
 *
 * @param contextName Context name
 * @param fn Context function
 */
export default function useContext<T extends (...args: any[]) => any>(contextName: string, fn: T) {
  type Context = ReturnType<T>

  const { useProvide, useInject: useStore } = createContext<Context>(contextName)

  function setupStore(...args: Parameters<T>) {
    const context: Context = fn(...args)
    return useProvide(context)
  }

  return {
    /** Setup store in the parent component */
    setupStore,
    /** Use store in the child component */
    useStore
  }
}

/** Create context */
function createContext<T>(contextName: string) {
  const injectKey: InjectionKey<T> = Symbol(contextName)

  function useProvide(context: T) {
    provide(injectKey, context)

    return context
  }

  function useInject() {
    return inject(injectKey) as T
  }

  return {
    useProvide,
    useInject
  }
}
