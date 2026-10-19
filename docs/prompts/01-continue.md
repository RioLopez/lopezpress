# 模板 01 —「继续」:恢复上下文 + 推进下一优先级任务

## 何时用
用户说「继续」「接着弄」「推进下一项」时,原文粘贴。

## 提示词正文(粘贴区)

```
按 lopezpress 的 AI 协作标准执行「继续」流程:

1. 读上下文(禁止跳过):
   - AGENTS.md(红线速查)
   - HANDOVER.md §0 和 §7(现状 + 陷阱)
   - PROGRESS.md 最新一轮(上次干到哪、验证基线是什么)
   - docs/TODO.md(唯一权威待办,挑最高优先级未完成项)
   - .workbuddy/memory/MEMORY.md 和最近 1-2 篇日志

2. 选定任务后,先分 3-5 行汇报:
   - 选中的任务(来自 TODO 第几项 / PROGRESS 遗留)
   - 任务级别(T0-T3)和计划流程
   - 预期改动文件清单

3. 按 docs/AI_STANDARD.md §3 五阶段执行;破坏性操作先列清单等确认。

4. 完成后必须:
   - 验证门禁五命令全绿(go build / go vet / go test / tsc --noEmit / vite build)
   - 行为验证贴证据(能跑的就跑,贴状态码/日志)
   - 更新 PROGRESS.md(轮次条目)、docs/TODO.md(勾销)、.workbuddy/memory 当日日志

5. 最终汇报:结论先行(做了什么/验证结果/遗留项),附证据。
```

## 预期产出
- 上下文恢复汇报(选中任务 + 理由)
- 按门禁验证的实现
- PROGRESS / TODO / memory 三处记录更新
