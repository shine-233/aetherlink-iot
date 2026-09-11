# 陈旧 worktree 副本的未提交改动存档

日期：2026-09-11。用途：在删除 7 个 `.claude/worktrees/agent-*` 陈旧副本前，
把它们相对旧提交 `c0d6fd4` 的未提交改动导出留档，避免内容丢失。

## 内容说明

- `<name>.patch`：`git diff HEAD` 导出的已跟踪文件改动。
- `<name>.status.txt`：`git status --short` 输出，含未跟踪文件清单。

## 为什么有两个目录没有 .patch

`agent-a2f86f89c8a9680c2` 与 `agent-af63016eae2839d35` **没有任何已跟踪文件的改动**，
`git diff HEAD` 为空，因此只有 `.status.txt`。
它们的未跟踪文件（`automation_tests/lib/runner/run-artifacts.js`、
`automation_tests/scripts/check_production_placeholders.js`）**在主干中均已存在**，
说明这些工作早已合并，删除副本不会丢失任何内容。

## 副本去向

7 个副本整体移出仓库到 `C:\Users\Zz\Documents\projects\archive\aetherlink-stale-worktrees-20260911`
（781MB），仓库内 `.claude/` 已归零，`git worktree list` 只剩 main。
该外部目录可安全删除。
