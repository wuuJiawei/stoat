# 风险例外策略

风险例外只允许对一个确定的 `item_id` 屏蔽指定正向规则，不支持按 label、通配符或任意脚本放行，也不能屏蔽 Apple 信任规则。

正式 Schema：[schemas/risk-policy.v1.schema.json](../schemas/risk-policy.v1.schema.json)

```json
{
  "schema_version": 1,
  "exceptions": [
    {
      "item_id": "0123456789abcdefabcd",
      "rule_ids": ["script-execution"],
      "reason": "已核验的内部维护脚本",
      "expires_at": "2026-12-31T00:00:00Z"
    }
  ]
}
```

使用：

```bash
stoat scan --rules policy.json
stoat snapshot --rules policy.json --output after.json
```

约束：

- JSON 严格校验，未知字段、未知规则、重复例外和非普通文件会被拒绝。
- 过期例外自动失效并产生 warning。
- 被屏蔽 finding 仍保留在 JSON 和详情页中，包含理由，便于审计。
- 例外只影响风险分数，不修改扫描事实、系统状态或快照内容。
