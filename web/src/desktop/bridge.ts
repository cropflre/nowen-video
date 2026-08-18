// 兼容入口：Desktop 2.0 的唯一实现已经迁移到 platform/desktop。
// 现有组件暂时继续从 src/desktop/bridge 导入，避免在同一阶段制造无意义的大范围改动。
export * from '../platform/desktop/bridge'
export { default } from '../platform/desktop/bridge'
