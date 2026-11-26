### Added

- `nsgotest`, `gpu`: add `nsgotest.TranslateShader` (`nsgotest.ShaderSources`) and `gpu.TranslateShaderModule` (`gpu.PipelineSources`) to translate fragment and 3D shaders headlessly across GPU backends with per-backend error reporting (#2266 by @owocc).

### Fixed

- `gpu`: recover shader translator panics and return `ErrShaderCompile` instead of crashing or enqueuing empty artifacts when no backend translates a pipeline in `CreatePipeline` (with a `PipelineCache`) or `CreatePipelineAsync` (#2266 by @owocc).
