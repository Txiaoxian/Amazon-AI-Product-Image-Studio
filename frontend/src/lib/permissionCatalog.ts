import type { UserAdminPermission } from '../types/userAdmin'

interface PermissionCopy {
  name: string
  description: string
}

const permissionCopyByCode: Record<string, PermissionCopy> = {
  'user:read': { name: '查看用户', description: '查看当前租户的用户列表与用户详情。' },
  'user:create': { name: '创建用户', description: '在当前租户中创建普通用户账号。' },
  'user:update': { name: '编辑用户', description: '修改当前租户用户的显示名称等安全字段。' },
  'user:disable': { name: '启用或禁用用户', description: '启用或禁用当前租户的用户账号。' },
  'role:read': { name: '查看角色与权限', description: '查看当前租户的角色以及系统权限目录。' },
  'role:manage': { name: '管理角色与权限', description: '创建、编辑、删除自定义角色并配置角色权限。' },
  'project:read': { name: '查看产品', description: '查看当前租户中有权限访问的产品。' },
  'project:create': { name: '创建产品', description: '在当前租户中创建新的产品。' },
  'project:update': { name: '编辑产品', description: '修改有权限管理的产品资料。' },
  'project:delete': { name: '删除产品', description: '删除有权限管理的产品。' },
  'project:member:manage': { name: '管理产品成员', description: '添加、修改或移除产品成员。' },
  'asset:read': { name: '查看产品素材', description: '查看有权限访问的产品素材。' },
  'asset:upload': { name: '上传产品素材', description: '向有编辑权限的产品上传参考图片。' },
  'asset:update': { name: '编辑产品素材', description: '修改产品素材的名称、分类和收藏状态。' },
  'asset:delete': { name: '删除产品素材', description: '删除有权限管理的产品素材。' },
  'asset:download': { name: '下载产品素材', description: '通过后端授权下载产品素材。' },
  'task:read': { name: '查看生图任务', description: '查看有权限访问的生图任务和生成记录。' },
  'task:create': { name: '创建生图任务', description: '使用已分配的模型创建生图或图片编辑任务。' },
  'task:cancel': { name: '取消生图任务', description: '取消有权限操作且尚未结束的生图任务。' },
  'task:retry': { name: '重试生图任务', description: '重试有权限操作且允许重试的生图任务。' },
  'provider:read': { name: '查看中转站', description: '仅供管理员查看当前租户的 AI 中转站配置。' },
  'provider:manage': { name: '管理中转站', description: '仅供管理员创建、编辑、删除、启用、禁用和测试 AI 中转站。' },
  'model:read': { name: '查看模型', description: '查看当前账号获准使用的 AI 模型及能力。' },
  'model:manage': { name: '管理模型', description: '仅供管理员创建、编辑、删除、启用或禁用 AI 模型配置。' },
  'usage:read': { name: '查看用量', description: '查看当前租户的模型调用用量与费用统计。' },
  'audit:read': { name: '查看审计记录', description: '查看当前租户的操作日志和接口调用记录。' },
  'system:settings:manage': { name: '管理系统设置', description: '查看和修改当前租户的运行参数与系统设置。' },
}

export function permissionCopy(permission: UserAdminPermission): PermissionCopy {
  return permissionCopyByCode[String(permission.code)] ?? {
    name: permission.name || String(permission.code),
    description: permission.description || permission.name || String(permission.code),
  }
}
