# 问题分级与上线判定

## 分级

| 级别 | 判定依据 | 上线处理 |
| --- | --- | --- |
| P0 | 严重安全、数据损坏或核心流程不可用等不可接受风险 | 必须修复并验证，不接受遗留解释豁免 |
| P1 | 明显功能错误、兼容性或稳定性风险，需要处理 | 修复并验证，或提供经评审确认合理的遗留解释 |
| P2 | 可改进项及可供参考的低风险问题 | 记录影响、风险和确认情况供参考，不作为上线阻断 |

按实际影响、触发条件和范围分级；不能因修复成本大而降级。原“阻塞/建议/Nitpick”不与 P0/P1/P2 机械一一映射，按上述依据重新判断。

## 结论计算顺序

| 条件 | 结论 | 上线评审门禁 |
| --- | --- | --- |
| 采集证据不足、覆盖未完成或计数输入无效 | 证据不足 | 不放行；列出缺口和已确认问题 |
| 证据/覆盖完整，未解决 P0 > 0 | 不通过 | 修复并验证 P0 |
| P0 = 0，但未解决且缺少合理解释的 P1 > 0 | 不通过 | 修复或完善合理解释 |
| P0 = 0，遗留 P1 均有合理解释 | 有条件通过 | 在报告列明每项解释、风险和缓解条件 |
| P0 = 0，未解决 P1 = 0 | 通过 | P2 保留为风险参考 |

此表中的 P0/P1 指未解决数量。报告同时保留发现总数与未解决数；已修复但未验证仍算未解决。证据不足时的数字仅代表已确认发现，不能写成完整范围的最终总数，更不能把未知填成 0。

通过是针对指定 SHA 和范围的上线评审意见，不能替代部署授权或环境验收。批量评审要求所有对象及跨模块检查完成；有失败/缺失对象时不能整体通过。

## 什么是合理的 P1 遗留解释

逐项记录：

- `reason`：本次不修复的具体原因及适用范围，必须与实际代码、配置或验证相符。
- `risk`：触发场景、影响对象和后果。
- `mitigation`：当前可用的规避措施或约束，例如功能开关保持关闭。
- `reasonable`：评审者基于以上证据判断合理后设为 true；不是“只要填了文字就自动合理”。

“后续再改”“暂时没时间”等没有风险边界的文字不算合理解释。证据不足不能用遗留解释豁免；P0 不能使用 P1 的解释机制。对外报告也要保留遗留 P1 和对应条件。

## 确定性判定工具

```bash
python3 <skill_dir>/scripts/review_decision.py /tmp/review-input.json --evidence /tmp/mr-evidence.json
```

输入示例：

```json
{
  "evidence_complete": true,
  "review_complete": true,
  "reviewed_files": ["src/app.py", "scripts/run.sh"],
  "supplemented_files": [
    {"path": "scripts/run.sh", "sha": "<head_sha>", "evidence": "git show 核验仅新增可执行权限，脚本内容未变", "verified": true}
  ],
  "findings": [
    {
      "id": "F-001",
      "severity": "P1",
      "status": "open",
      "justification": {
        "reason": "当前发布配置关闭该功能，启用前必须修复",
        "risk": "启用后该分支可能返回错误结果",
        "mitigation": "本次保持开关关闭，并核验发布配置",
        "reasonable": true
      }
    },
    {"id": "F-002", "severity": "P2", "status": "open"}
  ]
}
```

- 推荐通过 `--evidence <path>`（批量可重复传入）或在 JSON 中内嵌 `evidence` 绑定采集产物：工具将自动校验固定 `snapshot`（`base_sha`/`head_sha`）、发现项 `sha` 一致性及 `file_gaps` 补证闭环，并在输出中回显绑定的 `snapshot`。
- 若采集仅因单文件 `file_gaps`（二进制/空 diff/纯权限或重命名）返回 `evidence_complete=false`，须在 `supplemented_files` 中提供对应路径、固定 SHA、`evidence` 说明及 `verified: true`，工具核验全覆盖后才视为证据齐备；结构性或传输失败无法通过 `supplemented_files` 绕过。
- 提供 `reviewed_files` 时，工具自动比对证据中的全部变更文件列表，防止大 diff 分批读取时遗漏文件卻误判覆盖完成。批量评审可通过 `targets` 数组汇总各对象的 `evidence_complete`、`review_complete` 与 `head_sha`。
- `findings` 包含全部已确认发现，级别只接受 P0/P1/P2，状态只接受 open/fixed；提供 `id` 时强制校验唯一性以防跨批次重复累计。
- fixed 必须同时 `verified: true`，并在报告给出当前评审 SHA 对应的修复和验证证据，否则仍按未解决计数。
- 工具检查结构、计数、SHA 绑定和判定逻辑；评审者负责解释合理性、证据真实性和覆盖完整性。

输出包含 `conclusion`、`release_passed`、各级 `total/unresolved`、P1 `explained/unexplained`、`counts_complete` 和证据缺口。退出码：0 为通过或有条件通过，1 为不通过，2 为证据不足/输入读取失败。stdout 始终应作为结论来源；异常无输出时也不能放行。
