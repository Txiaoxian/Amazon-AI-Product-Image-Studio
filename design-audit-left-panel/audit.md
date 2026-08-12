# 生图左侧参数区交互审查与整改记录

日期：2026-08-12

## 审查目标

减少生图主流程中左侧长表单的纵向拖动，把“描述画面、添加参考、调整生成参数”按用户任务分组，同时保留现有模型能力、参考图、生成任务和 SSE 数据链路。

## 审查路径与健康度

1. **原页面：高级设置收起 — 一般**
   表单本身没有溢出（`clientHeight=598`、`scrollHeight=598`），但三类参考素材固定占据首屏，主任务“描述画面”被推到下方，所有输入仍堆叠在同一条长路径中。证据：[01-current-collapsed.png](./01-current-collapsed.png)
2. **原页面：展开高级设置 — 较差**
   设置展开后出现内层滚动（`clientHeight=598`、`scrollHeight=710`），模型与生成参数落到折叠区下方，用户必须滚动并反复确认上下文。证据：[02-current-advanced-open.png](./02-current-advanced-open.png)
3. **优化后：画面分区 — 良好**
   默认只保留提示词、三个常用快捷要求和当前参数摘要；表单与当前分区均无内层滚动（`598/598`、`516/516`）。证据：[03-optimized-prompt.png](./03-optimized-prompt.png)
4. **优化后：参考分区 — 良好**
   产品主体、构图、风格三种参考集中在独立分区，按需进入，不再阻挡提示词；表单与分区均无内层滚动。证据：[04-optimized-references.png](./04-optimized-references.png)
5. **优化后：参数分区 — 良好**
   模型、刷新、配置入口与模型支持的参数集中呈现；常用参数使用紧凑双列布局，模型缺失时直接提供“配置可用模型”。当前无模型状态无内层滚动。证据：[05-optimized-settings.png](./05-optimized-settings.png)
6. **移动端：画面分区 — 良好**
   390×844 视口无横向溢出，表单 `335/335`、分区 `253/253`，底部导航和主结果区保持可用。证据：[06-optimized-mobile.png](./06-optimized-mobile.png)

前后同视口对照：[07-before-after-comparison.png](./07-before-after-comparison.png)

## 竞品交互结论

- [Midjourney Web](https://docs.midjourney.com/hc/en-us/articles/33390732264589-Creating-on-Web) 把提示词作为主入口，图片与设置通过按需菜单打开；默认参数不会持续占据主画布。
- [Leonardo AI Image Generation](https://intercom.help/leonardo-ai/en/articles/8942360-how-to-generate-images-with-leonardo-ai) 让快速生成与更深配置分层，侧栏只保留关键设置；[Image Guidance](https://intercom.help/leonardo-ai/en/articles/8497988-image-guidance) 再根据参考图上下文显示角色和权重。
- [Adobe Firefly](https://helpx.adobe.com/au/firefly/web/generate-images-with-text-to-image/customize-generated-images/set-styles-for-image-generation.html) 只在选择参考后显示相关强度，降低无关参数噪音。
- [Ideogram Prompt Box](https://docs.ideogram.ai/using-ideogram/ui-overview/ui-components/prompt-box) 将模型、速度和数量收进提示词附近的选项栏，强调“先表达意图，再调整参数”。

共同模式不是简单地缩短表单，而是采用“主任务优先、设置按需展开、上下文条件显示、当前选择可回看”的渐进披露方式。

## 已实施方案

- 将左侧改为“画面 / 参考 / 参数”三段式任务页签，默认进入“画面”。
- 在画面底部固定显示当前模型、比例和生成张数摘要，点击“调整”直达参数。
- 参考页签显示已选数量；无可用模型时，参数页签显示“待选”并提供配置入口。
- 模型失效或提交时没有可用模型，会自动切换到“参数”，确保错误与修复入口可见。
- 参数按模型能力条件渲染，并以双列紧凑布局降低纵向占用。
- 页签使用 `tablist` / `tab` / `tabpanel` 语义，支持左右方向键、Home、End 和焦点跟随。
- 现有后端任务、参考素材、模型刷新、SSE 和生成提交逻辑未改动。

## 验证与限制

- 浏览器已逐项验证点击切换、参数摘要直达、键盘导航、桌面/移动端滚动状态；控制台无 error 或 warning。
- 回归测试覆盖三段式页签、键盘焦点、参考图选择、参数选择和模型失效自动跳转。
- 当前浏览器 QA 租户没有启用模型，因此参数截图展示的是空状态；完整模型参数由组件回归测试覆盖，未伪造线上配置。
