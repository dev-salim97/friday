# DevFlow 命令

读取命令后的软件交付请求并调用 `devflow` Skill。每阶段优先执行 `prepare --emit-prompt → start → finish`，仅在恢复已 prepare 阶段时单独生成 prompt。REQUIREMENT 由主 Agent 调用需求澄清 Skill，报告完成后必须展示摘要并等待用户明确确认，执行 `approve --stage REQUIREMENT --user-confirmed` 后才能继续；SUMMARY 由主 Agent 汇总已验证产物，这两个阶段都不创建子 Agent。实际 route 中其他阶段必须按 adapter 拓扑创建真实、独立的执行者并登记。
