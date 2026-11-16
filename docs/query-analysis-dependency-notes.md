# 查询分析依赖决策

## 决策

首个确定性的查询分析实现使用 Go 标准库处理显式时间布局，并使用 `golang.org/x/text` 处理 Unicode 归一化与与语言环境无关的大小写处理。它不引入通用自然语言日期解析器、分词器、在线模型客户端或查询规划提供方。

仓库已通过传递依赖解析到 `golang.org/x/text v0.31.0`。如果查询分析直接导入其 `cases` 或 `unicode/norm` 包，则必须通过 `go mod tidy` 将该模块提升为直接依赖，以使所有权明确。

## 包审阅

以下包页面于 2026-09-11 在 pkg.go.dev 上审阅：

- Go 标准库 `strings` 与 `unicode`：稳定、BSD-3-Clause，是被广泛导入的基础组件，用于确定性 rune 分类与显式分隔符处理。它们足以支持第一个策略的有界词项抽取，其规则必须由仓库自有并版本化。
- `github.com/blevesearch/segment` `v0.9.1`：稳定、Apache-2.0，在审阅页面上被 164 个包导入。它是一个紧凑的 Unicode 文本分段器，但其通用词边界行为并不能消除对显式混合语言策略规则的需要。
- `github.com/ikawaha/kagome/v2/tokenizer` `v2.11.0`：稳定、MIT，在审阅页面上被 103 个包导入。其日语形态学词典与特定语言的运行时表面对于初始的语言无关有界策略而言并不必要。
- `golang.org/x/text/cases`：稳定模块、BSD-3-Clause。它提供确定性的、Unicode 感知的大小写映射，无需远程服务。
- `golang.org/x/text/unicode/norm`：稳定模块、BSD-3-Clause。它提供比较与去重有界词项所需的 Unicode 归一化形式。
- `github.com/araddon/dateparse`：稳定包、MIT，但其宽泛的启发式日期识别比本次发布（rollout）所需更大，且策略性更弱。
- `github.com/markusmobius/go-dateparser`：稳定模块、BSD-3-Clause，但引入了通用自然语言解析器与传递表面，对首个有界确定性策略而言并不必要。

## 理由与验证

显式的已接受布局以及固定的时钟/时区使时间行为可版本化且可回放。Unicode 归一化与大小写处理解决了一个狭窄的标准问题，而实体、意图与多跳规则仍由仓库自有的策略决定。这避免了宽泛启发式解析器带来的静默行为变化，并将格式错误或模棱两可的日期保留在显式的未知/回退类别中。

仓库在 `scripts/` 或产品验证工作流中没有专门的依赖许可证检查器。因此该决策通过可用的仓库与 Go 控制手段验证：`go list -m -json golang.org/x/text` 解析出已有校验和的 `v0.31.0` 模块，`go mod graph` 确认了其模块所有权，`go mod download -json golang.org/x/text@v0.31.0` 报告了与 `go.sum` 中记录相同的模块与 `go.mod` 校验和。也尝试了完整的 `go mod verify`，但它失败了，因为机器范围的 Go 缓存中包含一个已存在的、被修改的 `github.com/docker/docker v28.3.3+incompatible` 目录。该模块不是本次变更引入或选择的依赖；该失败作为环境发现保留，而不是被描述为通过的仓库检查。

没有新增分词器或日期解析器模块，因此没有新的传递许可证表面。一旦 `x/text` 被直接导入，`go mod tidy` 必须将其从间接提升为直接，并且必须重新运行模块、测试、仓库质量命令以及干净缓存的 `go mod verify`。

任何未来的解析器依赖都需要策略版本变更，以及对维护性、许可证、确定性行为与传递依赖的新一轮审阅。
