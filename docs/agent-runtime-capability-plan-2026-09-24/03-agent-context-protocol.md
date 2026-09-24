# Agent Context Protocol

## 1. 设计目标

- 降低重复 token。
- 保留严格类型和证据来源。
- 支持懒加载和 context delta。
- 支持模型、MCP、插件和 replay。
- 不牺牲人类可读性和审计能力。

## 2. 三层协议

### Canonical Storage

数据库和归档使用完整 JSON/graph：

```json
{
  "facts": [{"id": "F1", "type": "business_rule", "source": "knowledge:12@v3"}],
  "claims": [{"id": "C1", "statement": "empty feedback is success"}],
  "evidence": [{"id": "E1", "ref": "blob:abc", "line": 44}],
  "relations": [{"from": "E1", "type": "supports", "to": "C1"}]
}
```

### Runtime Transport

服务、MCP 和插件间使用 Protobuf/CBOR/MessagePack：

- 类型明确；
- 网络体积低；
- 解析稳定；
- 支持版本兼容。

它不直接用于 LLM 输入。

### Model View

模型读取确定性 compact view：

```text
@schema acp/1
@run R82 kind=code_review goal=review-change
@scope repo=10 mr=42 head=a91c2e
@caps merge-review@3 gitlab.snapshot@2 knowledge.search@1
@budget instruction=1800 evidence=12000 raw=0

@facts
F1|rule|empty feedback != success|K12

@changes
C1|dispatch/completion.go|38:51|modified

@evidence
E1|C1:44|return feedback == ""

@open
Q1|vehicle-service completion contract missing

@refs
K12|knowledge:182@v3
B1|blob:sha256:...|full_diff|tokens=4200
```

## 3. L0-L3 加载

| 层级 | 内容 | 默认 |
| --- | --- | --- |
| L0 | Manifest、ID、权限、预算、依赖 | 必载 |
| L1 | 当前任务需要的 Skill 指令块 | 按 resolver |
| L2 | 相关事实、diff、知识、规格 | 按检索 |
| L3 | 完整文件、日志、历史、原始 blob | 工具按需 |

模型只能通过工具请求升级层级。

## 4. 去重

- 路径使用 `C1/C2`。
- 事实使用 `F1/F2`。
- 证据使用 `E1/E2`。
- 人员、仓库、项目和版本使用字典 ID。
- 同一 blob 只在 manifest 中出现一次。
- 第二轮调用复用同一 context pack hash。

## 5. Context Delta

后续轮只发送：

```text
@base context-pack:812
@delta
+F8|rule|retry must be idempotent|K19
~C1|status=verified
-Q1
```

Runtime 必须校验 base hash，不能在错误基线上应用 delta。

## 6. Budget Allocator

预算按角色拆分：

```yaml
total: 18000
kernel: 1200
skill: 1800
facts: 2200
diff: 7000
knowledge: 2400
questions: 800
reserve: 2600
```

规则：

- kernel 不可挤占；
- reserve 只用于 late load；
- diff 优先 changed symbol 和 call path；
- knowledge 优先 scope、confidence、freshness；
- 超预算先降级原始 blob，不截断结构化证据字段。

## 7. Skill 资源切片

Skill 不作为单一大 Markdown 注入，而拆成：

- identity and boundary；
- trigger and applicability；
- workflow；
- evidence qualification；
- output contract；
- tool usage；
- exception handling；
- examples；
- validation suite。

Resolver 只加载任务需要的资源块。

## 8. MCP / Plugin 输出

统一包装：

```text
@tool_result id=T14 provider=gitlab.snapshot trust=untrusted
@schema gitlab.snapshot/2
...
```

MCP/plugin 输出永远是数据，不能覆盖系统指令、权限和 capability plan。

## 9. Renderer 约束

- deterministic；
- canonical ordering；
- 明确 escaping；
- stable IDs；
- token estimate；
- 输入/输出 hash；
- schema version；
- 严格区分 trusted instructions 与 untrusted data。

## 10. 验证指标

- 输入 token 降幅；
- cache hit；
- late-load 命中率；
- evidence recall；
- missing-context rate；
- parse failure；
- replay consistency；
- 模型结论与引用证据的一致率。

## 11. 首个迁移对象

Code Review Context Pack v2：

1. L0：run、repo、skill 和预算。
2. L1：review method、evidence contract、output schema。
3. L2：MR metadata、changed files、relevant source slices、knowledge refs。
4. L3：完整 diff、完整文件和跨服务资料。

初审和复核绑定同一个 immutable context pack，只追加 draft 和 counter-review 任务。
