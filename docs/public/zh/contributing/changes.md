---
title: 提交改动
description: 向 NSGo 提交 pull request 需要什么：每个后端的同样改动、测试、文档和一行更新日志。
group: 参与项目
order: 82
---

## 开始之前

比修复更大的改动，请先开一个 issue 或 discussion，在写代码之前就设计达成一致。设计中尚未定论的问题列在 [ARCHITECTURE.md](https://github.com/nsgo-dev/nsgo/blob/main/ARCHITECTURE.md#open-questions) 中，先在那里定下答案。

## 每个后端

一个后端里的修复或功能，通常其他后端也需要。在每个需要的目标上做同样的改动，或者把缺少的部分记到[路线图](https://github.com/nsgo-dev/nsgo/blob/main/docs/roadmap.md)里。组件在每个平台上的外观和行为都应一致。

## 检查

使用 `nsgo` 工具，而不是 `go vet`、`go test` 或 `go build`：

```sh
go tool nsgo check   # 检查所有包
go tool nsgo test    # 运行所有包的测试
```

为改动加上测试：用户看到的内容用 `nsgotest`，其余用包自己的测试。在改动涉及的平台上运行 Gallery（[开发环境](../contributing.md#gallery)）。

用 `go.mod` 中 Go 版本自带的 `gofmt` 格式化 Go 文件；`gofmt -l .` 会列出需要格式化的文件。CI 会检查所有纳入版本库的 Go 文件，有未格式化的文件时失败并列出文件名。

## Pull request

标题使用 Conventional Commits 格式，例如 `fix(linux): keep the caret after a paste`：squash 合并时标题会成为提交信息，格式不对时检查会失败。模板会要求写明改动涉及的目标平台、可见改动的前后截图，以及实际跑过的检查。

在你推送的提交上运行 `go tool nsgo check` 和 `go tool nsgo test`，然后填写模板里的 `Local CI` 块：这个提交、它所基于的 `main` 提交、每条命令与退出码（写成 `→ N`，可以写在一行，也可以写成嵌套列表）、结果，以及本地跑不了的部分。`pr-triage.yml` 按该块给 pull request 打标签：`local-ci-pass`、`local-ci-fail`，或者在块声明跳过了检查，或块缺失、不完整、写的是另一个提交、声明 `pass` 却有非零退出码时打 `local-ci-skipped`；机器人评论会列出块无效的原因，被打 `local-ci-skipped` 或 `local-ci-fail` 的 pull request 由维护者在合并前审计。Dependabot 等机器人的 pull request 不打这个标签。声明是承诺，不是证据：能否合入仍由必需的检查决定。

## 文档与更新日志

- 更新事实发生变化的文档：NSGo 如何工作写在 `docs/architecture/` 的专题里，应用和组件如何使用它写在 `docs/public/` 的页面里（[文档与网站](docs.md)）。
- `README.md` 和 `README.zh-CN.md` 保持一致；其余内容都用英文，中文页面位于 `docs/public/zh/`。
- 用户能看到的改动记一行，并附上引用，例如 `(#12 by @you)`，写在 `changelog.d/` 下单独的文件里、所属类型的标题下（例如 `### Fixed`），不要直接改 `CHANGELOG.md`：发版时由这些文件生成（[格式](https://github.com/nsgo-dev/nsgo/blob/main/changelog.d/README.md)）。
