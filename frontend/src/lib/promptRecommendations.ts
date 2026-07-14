import type { WorkbenchImageType } from '../types/workbench'

export interface PromptRecommendation {
  id: string
  title: string
  description: string
  prompt: string
}

const SOURCE_RULE = '主体依据：优先以已上传参考图中的实际售卖产品为唯一视觉依据；未提供参考图时，严格依据用户补充的产品描述。'
const IDENTITY_RULE = '必须保持：产品的形状、结构、比例、颜色、材质、表面纹理、标签位置和真实配件关系，不重新设计产品，不增加未售卖的配件。'
const FACT_RULE = '事实约束：尺寸、功能、材质、认证、性能数据、优惠和对比结论只能使用用户明确提供的信息；信息缺失时不得猜测或编造。'
const OUTPUT_RULE = '输出要求：专业商业摄影，产品边缘清晰，细节真实，无水印、无乱码、无无关品牌或第三方商标。'

function structuredPrompt(...sections: string[]): string {
  return sections.join('\n')
}

export const PROMPT_RECOMMENDATIONS: Readonly<Record<WorkbenchImageType, readonly PromptRecommendation[]>> = {
  MAIN: [
    {
      id: 'main-white-background',
      title: '合规纯白主图',
      description: '纯白背景、完整商品、无文字与道具',
      prompt: structuredPrompt(
        '用途：Amazon 商品主图。',
        SOURCE_RULE,
        '构图：单一售卖产品完整居中展示，正面略带三分之四视角，主体约占画面 85%，四周保留均匀安全边距，不裁切产品。',
        '背景与光线：纯白背景 RGB(255,255,255)，柔和均匀的专业棚拍光，准确还原颜色，只有自然且克制的接触阴影。',
        IDENTITY_RULE,
        '禁止：任何文字、图标、Logo 叠加、水印、边框、拼图、装饰道具、人物、场景背景、未包含在售卖内容中的物品。',
        OUTPUT_RULE,
      ),
    },
    {
      id: 'main-front-view',
      title: '标准正面主图',
      description: '正面视角，适合包装规整或结构对称商品',
      prompt: structuredPrompt(
        '用途：Amazon 商品主图。',
        SOURCE_RULE,
        '构图：严格正面视角，镜头与产品中心齐平，产品垂直端正、完整可见，主体约占画面 85%，轮廓清楚且不发生透视畸变。',
        '背景与光线：纯白背景 RGB(255,255,255)，高显色柔光棚拍，表面高光受控，保留真实材质层次。',
        IDENTITY_RULE,
        '禁止：文字、说明线、尺寸、徽章、额外 Logo、水印、边框、道具、人物、非售卖配件和场景化背景。',
        OUTPUT_RULE,
      ),
    },
    {
      id: 'main-bundle',
      title: '套装内容主图',
      description: '仅展示实际包含的套装与配件',
      prompt: structuredPrompt(
        '用途：Amazon 套装商品主图。',
        SOURCE_RULE,
        '构图：将用户明确说明为售卖内容的主产品和配件整齐分层陈列，主产品保持视觉中心，全部物品完整可见，整体约占画面 85%。',
        '背景与光线：纯白背景 RGB(255,255,255)，一致的专业棚拍光线和自然接触阴影，物品之间不重叠遮挡关键结构。',
        IDENTITY_RULE,
        '禁止：添加参考图或用户说明中未明确包含的配件、包装赠品、文字、图标、拼图、水印、边框或场景道具。',
        OUTPUT_RULE,
      ),
    },
  ],
  A_PLUS: [
    {
      id: 'a-plus-feature-banner',
      title: '核心卖点横幅',
      description: '产品视觉与单一核心卖点的 A+ 模块',
      prompt: structuredPrompt(
        '用途：Amazon A+ 商品描述的横幅模块。',
        SOURCE_RULE,
        '画面概念：围绕一个用户明确提供的核心卖点构建高品质横向商业视觉，产品清楚突出，背景与品类和目标人群一致。',
        '构图：产品位于画面一侧，另一侧预留干净的文案安全区，层级简洁，不拥挤。',
        '文字：仅在用户提供精确文案时使用，逐字显示“[请填写核心标题]”和“[请填写一句说明]”；否则保留无字安全区。',
        IDENTITY_RULE,
        FACT_RULE,
        OUTPUT_RULE,
      ),
    },
    {
      id: 'a-plus-brand-story',
      title: '品牌故事横幅',
      description: '品牌理念、使用价值与真实生活方式',
      prompt: structuredPrompt(
        '用途：Amazon A+ 品牌故事模块。',
        SOURCE_RULE,
        '画面概念：用真实、克制的生活方式场景表达用户提供的品牌理念和产品价值，让产品自然融入目标用户的日常，不做夸张戏剧化处理。',
        '构图：宽幅叙事构图，产品仍清晰可识别，人物如出现应自然使用产品且不遮挡关键结构，预留品牌文案区域。',
        '文字与品牌：只使用用户明确提供的品牌名称、Logo 和精确文案；未提供时不生成替代品牌或虚构口号。',
        IDENTITY_RULE,
        FACT_RULE,
        OUTPUT_RULE,
      ),
    },
    {
      id: 'a-plus-steps',
      title: '使用步骤模块',
      description: '用清晰步骤降低理解和使用门槛',
      prompt: structuredPrompt(
        '用途：Amazon A+ 使用步骤说明模块。',
        SOURCE_RULE,
        '信息结构：依据用户提供的真实操作流程展示 3 个连续步骤，分别为“[步骤 1]”“[步骤 2]”“[步骤 3]”，每一步只呈现一个动作。',
        '构图：统一视角、背景和光线，步骤区块按从左到右排列，操作部位清晰，不遮挡产品，留出简短说明文字的位置。',
        '文字：步骤名称必须逐字使用用户提供的文案；没有明确步骤时只生成无文字的视觉分镜，不虚构操作方法。',
        IDENTITY_RULE,
        FACT_RULE,
        OUTPUT_RULE,
      ),
    },
  ],
  SCENE: [
    {
      id: 'scene-home',
      title: '家居使用场景',
      description: '真实家庭环境中的自然使用状态',
      prompt: structuredPrompt(
        '用途：Amazon 附图中的真实家居使用场景。',
        SOURCE_RULE,
        '场景：根据产品用途放置在整洁、可信的现代家庭环境中，环境尺度与产品真实尺寸一致，配套物品只用于说明使用情境。',
        '构图与光线：产品为明确视觉主体，三分之四视角，中景构图，自然窗光结合柔和补光，画面温暖但颜色准确。',
        '人物：仅在能真实说明使用方式时出现，动作自然，手部与产品接触关系正确，不遮挡关键功能。',
        IDENTITY_RULE,
        FACT_RULE,
        OUTPUT_RULE,
      ),
    },
    {
      id: 'scene-work',
      title: '办公专业场景',
      description: '整洁办公或专业工作环境',
      prompt: structuredPrompt(
        '用途：Amazon 附图中的办公或专业使用场景。',
        SOURCE_RULE,
        '场景：将产品置于与其用途匹配的整洁办公桌、工作室或专业环境中，背景真实可信，环境元素简洁且不抢夺注意力。',
        '构图与光线：产品位于清晰焦点，保留适度环境信息说明使用方式；采用自然侧光和柔和轮廓光，材质与接口细节可辨。',
        '使用关系：所有连接、摆放与人物操作都必须符合用户提供的实际使用方式。',
        IDENTITY_RULE,
        FACT_RULE,
        OUTPUT_RULE,
      ),
    },
    {
      id: 'scene-outdoor',
      title: '户外使用场景',
      description: '自然环境中的真实使用与尺度关系',
      prompt: structuredPrompt(
        '用途：Amazon 附图中的户外生活方式场景。',
        SOURCE_RULE,
        '场景：依据产品真实用途选择可信的户外环境，天气、地面和周边物体符合常识，产品尺寸和承重关系真实。',
        '构图与光线：产品保持主要视觉焦点，环境用于说明使用情境；使用自然日光、真实阴影和克制景深，避免电影海报式过度特效。',
        '人物：如出现人物，只展示自然且安全的真实操作动作，不暗示用户未提供的防水、耐候或安全能力。',
        IDENTITY_RULE,
        FACT_RULE,
        OUTPUT_RULE,
      ),
    },
  ],
  DETAIL: [
    {
      id: 'detail-material',
      title: '材质纹理特写',
      description: '突出表面材质、纹理和做工',
      prompt: structuredPrompt(
        '用途：Amazon 附图中的材质细节特写。',
        SOURCE_RULE,
        '镜头：微距或近距离商业摄影，聚焦用户指定的材质区域，真实呈现纹理、涂层、织法、颗粒或光泽变化。',
        '构图与光线：局部细节占画面主体，同时保留足够产品轮廓帮助识别；使用掠射柔光突出真实质感，不进行过度磨皮或锐化。',
        IDENTITY_RULE,
        FACT_RULE,
        OUTPUT_RULE,
      ),
    },
    {
      id: 'detail-structure',
      title: '关键结构特写',
      description: '接口、开关、连接件或功能结构',
      prompt: structuredPrompt(
        '用途：Amazon 附图中的关键结构特写。',
        SOURCE_RULE,
        '镜头：近距离展示用户指定的接口、开关、扣件、铰链、密封结构或操作部位，结构关系与参考图完全一致。',
        '构图与光线：关键部位清晰居中，采用专业柔光和适度景深，边缘、缝隙与连接关系清楚，背景简洁。',
        '说明空间：可预留一处干净区域供后续添加说明，但不要自行生成箭头、名称或功能结论。',
        IDENTITY_RULE,
        FACT_RULE,
        OUTPUT_RULE,
      ),
    },
    {
      id: 'detail-craft',
      title: '工艺细节特写',
      description: '接缝、边缘、车线或精密做工',
      prompt: structuredPrompt(
        '用途：Amazon 附图中的工艺做工特写。',
        SOURCE_RULE,
        '镜头：专业微距摄影，突出用户指定的接缝、倒角、车线、边缘处理或装配精度，保留真实微小纹理。',
        '构图与光线：使用侧向柔光呈现立体层次，焦点准确落在工艺区域，色彩中性，背景不干扰。',
        IDENTITY_RULE,
        FACT_RULE,
        OUTPUT_RULE,
      ),
    },
  ],
  DIMENSION: [
    {
      id: 'dimension-three-view',
      title: '三视图尺寸标注',
      description: '正面、侧面、俯视与真实尺寸线',
      prompt: structuredPrompt(
        '用途：Amazon 商品尺寸信息图。',
        SOURCE_RULE,
        '版式：在干净浅色背景上展示同一产品的正面、侧面和俯视三个一致比例的视图，排列整齐，产品几何结构保持准确。',
        '尺寸标注：只使用用户提供的数值，分别标注“[宽度]”“[高度]”“[深度]”及明确单位；尺寸线纤细、端点准确、文字清晰可读。',
        '事实约束：未填写或不确定的尺寸必须省略，不估算、不推断、不补全；不同视图不得改变产品比例。',
        IDENTITY_RULE,
        OUTPUT_RULE,
      ),
    },
    {
      id: 'dimension-front',
      title: '正面尺寸图',
      description: '单一视图突出宽高尺寸',
      prompt: structuredPrompt(
        '用途：Amazon 商品正面尺寸信息图。',
        SOURCE_RULE,
        '版式：产品严格正面、垂直居中展示，背景干净，轮廓清楚，画面两侧留出尺寸线空间。',
        '尺寸标注：仅显示用户提供的“[宽度] × [高度] [单位]”，尺寸线与对应边缘平行，数字和单位清晰，不与产品重叠。',
        '事实约束：任何未由用户明确提供的尺寸都不显示，不添加重量、容量、性能或认证信息。',
        IDENTITY_RULE,
        OUTPUT_RULE,
      ),
    },
    {
      id: 'dimension-space-fit',
      title: '空间适配示意',
      description: '用真实场景帮助理解尺寸和摆放空间',
      prompt: structuredPrompt(
        '用途：Amazon 商品空间适配尺寸图。',
        SOURCE_RULE,
        '场景：将产品放入用户指定的真实使用空间，产品与环境物体保持正确尺度，并以简洁尺寸线说明关键占地范围。',
        '尺寸标注：只逐字使用用户提供的“[关键尺寸] [单位]”和“[适配空间说明]”；没有明确数据时仅保留无文字场景，不虚构尺寸。',
        '构图：环境简洁，产品清晰，尺寸线不遮挡关键结构，不使用夸张透视制造错误大小印象。',
        IDENTITY_RULE,
        FACT_RULE,
        OUTPUT_RULE,
      ),
    },
  ],
  SELLING_POINT: [
    {
      id: 'selling-point-single',
      title: '单一核心卖点',
      description: '一张图只解释一个最重要的购买理由',
      prompt: structuredPrompt(
        '用途：Amazon 附图中的单一核心卖点图。',
        SOURCE_RULE,
        '信息重点：只表现用户明确提供的一个核心卖点“[核心卖点]”，通过真实产品细节或使用情境直观说明，不加入其他竞争信息。',
        '构图：产品占主要视觉权重，卖点对应部位清楚，预留简洁标题区，画面层级一眼可读。',
        '文字：仅逐字使用用户提供的标题“[标题]”和一句说明“[说明]”；未提供精确文案时不生成文字。',
        IDENTITY_RULE,
        FACT_RULE,
        OUTPUT_RULE,
      ),
    },
    {
      id: 'selling-point-three',
      title: '三卖点信息图',
      description: '围绕产品组织三个简洁功能点',
      prompt: structuredPrompt(
        '用途：Amazon 附图中的三卖点信息图。',
        SOURCE_RULE,
        '信息结构：以产品为视觉中心，围绕用户提供的“[卖点 1]”“[卖点 2]”“[卖点 3]”设置三个清晰区域，每个区域只配一个真实细节或简洁图形提示。',
        '构图：信息层级明确、留白充足，连接线不交叉，不遮挡产品，避免密集图标和长段文字。',
        '文字：所有标题和数据必须逐字来自用户输入；缺少某个卖点时减少区块数量，不自动补齐。',
        IDENTITY_RULE,
        FACT_RULE,
        OUTPUT_RULE,
      ),
    },
    {
      id: 'selling-point-benefit',
      title: '功能价值场景',
      description: '把真实功能转化为可理解的使用价值',
      prompt: structuredPrompt(
        '用途：Amazon 附图中的功能价值卖点图。',
        SOURCE_RULE,
        '画面概念：在真实使用场景中展示用户明确提供的功能如何解决“[用户问题]”，产品操作方式与结果必须可信。',
        '构图：使用前景产品和简洁场景形成明确视觉路径，预留标题区，避免过度戏剧化的特效或无法证实的结果。',
        '文字：仅使用用户提供的“[功能标题]”和“[价值说明]”，不使用“最佳”“第一”“100%”等未经证明的绝对化表述。',
        IDENTITY_RULE,
        FACT_RULE,
        OUTPUT_RULE,
      ),
    },
  ],
  PROMOTION: [
    {
      id: 'promotion-brand',
      title: '品牌宣传海报',
      description: '品牌调性、产品主视觉与简洁文案区',
      prompt: structuredPrompt(
        '用途：电商品牌宣传主视觉。',
        SOURCE_RULE,
        '创意方向：依据用户提供的品牌定位、目标人群和色彩规范，制作高品质产品海报；产品保持真实，背景和光效服务于品牌气质。',
        '构图：产品是唯一主角，画面具有明确焦点和充足留白，预留品牌名称、标题和一句说明的位置。',
        '文字与品牌：仅使用用户提供的 Logo、品牌名和精确文案；未提供时不生成替代品牌、虚构徽章或伪造背书。',
        IDENTITY_RULE,
        FACT_RULE,
        OUTPUT_RULE,
      ),
    },
    {
      id: 'promotion-seasonal',
      title: '节日主题宣传',
      description: '克制的节日氛围，不遮挡产品',
      prompt: structuredPrompt(
        '用途：电商节日或季节主题宣传图。',
        SOURCE_RULE,
        '创意方向：围绕用户指定的“[节日或季节]”加入克制、品类相关的氛围元素，保持高级商业摄影质感，不让装饰遮挡产品。',
        '构图与光线：产品清晰居中或位于视觉黄金分割点，装饰元素形成引导，预留活动文案安全区。',
        '文字与优惠：只有用户提供精确活动名称、日期、价格或折扣时才显示；不得虚构促销、倒计时、评分或优惠力度。',
        IDENTITY_RULE,
        FACT_RULE,
        OUTPUT_RULE,
      ),
    },
    {
      id: 'promotion-launch',
      title: '新品发布视觉',
      description: '现代、简洁、有发布感的产品海报',
      prompt: structuredPrompt(
        '用途：电商新品发布宣传图。',
        SOURCE_RULE,
        '创意方向：现代、简洁、有层次的新品发布视觉，使用受控的背景色块、光影或材质舞台突出产品，不修改产品本身。',
        '构图：产品轮廓完整，主次关系明确，保留标题“[新品名称]”和副标题“[一句发布文案]”的安全区。',
        '文字与信息：仅逐字使用用户提供的产品名、上市信息和文案；不生成虚构奖项、认证、销量或用户评价。',
        IDENTITY_RULE,
        FACT_RULE,
        OUTPUT_RULE,
      ),
    },
  ],
  COMPARISON: [
    {
      id: 'comparison-models',
      title: '系列型号对比',
      description: '对比自有系列的真实规格与适用差异',
      prompt: structuredPrompt(
        '用途：Amazon 商品系列型号对比图。',
        SOURCE_RULE,
        '版式：将用户提供的系列型号按统一视角、统一比例和统一光线并列展示，列标题分别为“[型号 A]”“[型号 B]”“[型号 C]”。',
        '对比项目：只使用用户明确提供的尺寸、容量、配件或适用场景，行数保持精简，差异一眼可读。',
        '事实与品牌：不添加未提供的型号、规格、价格或结论，不使用第三方品牌、商标或产品外观。',
        IDENTITY_RULE,
        OUTPUT_RULE,
      ),
    },
    {
      id: 'comparison-alternative',
      title: '同类方案对比',
      description: '中性展示产品与常见方案的事实差异',
      prompt: structuredPrompt(
        '用途：Amazon 商品与常见同类方案的对比信息图。',
        SOURCE_RULE,
        '版式：左右分栏，左侧为“本产品”，右侧为用户定义的“[常见方案]”；采用相同尺度和中性视觉条件，避免通过大小或光线误导。',
        '对比内容：仅展示用户能够证实的“[差异 1]”“[差异 2]”“[差异 3]”，语言客观，不贬低竞品。',
        '禁止：第三方品牌名称、商标、包装或可识别产品外观；未经证实的性能结论、评分、排名和绝对化用语。',
        IDENTITY_RULE,
        FACT_RULE,
        OUTPUT_RULE,
      ),
    },
    {
      id: 'comparison-before-after',
      title: '使用前后对比',
      description: '在一致条件下展示真实可证明的变化',
      prompt: structuredPrompt(
        '用途：Amazon 商品使用前后对比图。',
        SOURCE_RULE,
        '版式：左右并列的“使用前”和“使用后”，使用相同镜头、角度、光线、距离和环境，唯一变化是用户明确提供且可证明的使用结果。',
        '构图：产品与作用对象关系真实，变化清晰但不过度夸张，标题和说明区域简洁。',
        '事实约束：不得制造用户未提供的清洁、修复、健康、美容或性能效果；结果不明确时不要生成前后差异。',
        IDENTITY_RULE,
        OUTPUT_RULE,
      ),
    },
  ],
}

export function getPromptRecommendations(imageType: WorkbenchImageType): readonly PromptRecommendation[] {
  return PROMPT_RECOMMENDATIONS[imageType]
}
