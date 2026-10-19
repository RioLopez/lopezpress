# 模板 02 — 新功能:spec → plan → 实现

## 何时用
新增端点/页面/数据模型变更/权限或可见性规则变化(T2 任务)。小改动(文案、格式)不要用本模板,直接做。

## 提示词正文(粘贴区)

```
按 echo-content-api 的 AI 协作标准实现新功能。

【任务描述】
<在这里写:要做什么、解决什么问题、边界是什么>

【执行要求】
1. 先读上下文:AGENTS.md、HANDOVER.md §7、PROGRESS.md 最新一轮、相关既有 spec(docs/specs/ 里找最接近的)、docs/decisions/ 相关决策。
2. 写 spec(不写码):按 docs/specs/ 既有格式 + EARS 验收语法(WHEN … THEN …),
   状态标 Draft;范围限定在我上面描述的功能,不自行扩大。
   同时在回复里给分步 plan(每步:改哪些文件、做什么、怎么验)。
3. 等我确认 spec 和 plan 后,再按计划分步实现。
4. 实现纪律:
   - 权限/可见性/角色判定走 services/roles.go 和 canSeePost,不散落 if-else
   - 新错误契约用英文;新界面文案加 i18n 中英双语键
   - 测试与实现同轮:service 层补用例,handler 层补路由级用例
   - 不引入新依赖(除非充分理由且我同意)
5. 完成后:验证门禁五命令全绿 + 行为验证贴证据 + 更新 PROGRESS/TODO/memory。
```

## 预期产出
- spec 文件(`docs/specs/NN-xxx.md`,格式照既有)+ 分步 plan
- 用户确认后的完整实现 + 测试
- 三处记录更新(PROGRESS 轮次 / TODO / memory 日志)
