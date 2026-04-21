/* *
 * @Author: chengjiang
 * @Date: 2026-04-21
 * @Description: 场景元信息结构,字段严格对齐 backend/internal/scenarios/template.md 的 Schema
**/
package scenario

// Scenario 场景元信息(硬编码注册表条目)
type Scenario struct {
	// 基础
	Slug          string
	Version       string
	Title         string
	Summary       string
	DescriptionMd string

	// 分类
	Category         string // frontend / backend / ops / devops / database / network / security / guide
	Difficulty       uint32 // 1-5
	EstimatedMinutes uint32

	// 用户画像
	TargetPersonas  []string
	ExperienceLevel string // intern / junior / mid / senior / expert

	// 知识维度
	TechStack []string
	Skills    []string
	Commands  []string
	Tags      []string

	// 运行配置
	Runtime RuntimeConfig

	// 判题配置
	Grading GradingConfig

	// 提示(Week 1 仅保留外壳,不下发内容)
	Hints []Hint

	// 元信息
	IsPremium bool
}

// RuntimeConfig 容器运行时参数
type RuntimeConfig struct {
	Image              string
	MemoryMB           int64
	CPUs               float64
	IdleTimeoutMinutes int
	PassedGraceMinutes int
	NetworkMode        string // none / isolated / internet-allowed
	Variants           []string
	// Security 容器安全加固,按场景按需开启(默认已 cap-drop=ALL + no-new-privileges)
	Security SecurityConfig
}

// SecurityConfig 映射到 runtime.SecuritySpec,控制容器额外 cap / 只读根文件系统
type SecurityConfig struct {
	CapAdd         []string // 例如 []string{"NET_BIND_SERVICE"}
	ReadonlyRootFS bool     // 开启后 --read-only + tmpfs(只写得到 /tmp 和 /home/player)
	TmpfsSizeMB    int      // tmpfs 大小,0 取默认 64
}

// GradingConfig 判题脚本参数
type GradingConfig struct {
	CheckScript         string
	CheckTimeoutSeconds int
	SuccessOutput       string // 默认 "OK"
}

// Hint 提示(Week 1 仅作为外壳)
type Hint struct {
	Level   uint32
	Content string
}

// Brief 列表页用的精简视图
type Brief struct {
	Slug             string
	Title            string
	Summary          string
	Category         string
	Difficulty       uint32
	EstimatedMinutes uint32
	TargetPersonas   []string
	TechStack        []string
	Tags             []string
	IsPremium        bool
}

// ToBrief 转换为列表项
func (s *Scenario) ToBrief() *Brief {
	return &Brief{
		Slug:             s.Slug,
		Title:            s.Title,
		Summary:          s.Summary,
		Category:         s.Category,
		Difficulty:       s.Difficulty,
		EstimatedMinutes: s.EstimatedMinutes,
		TargetPersonas:   s.TargetPersonas,
		TechStack:        s.TechStack,
		Tags:             s.Tags,
		IsPremium:        s.IsPremium,
	}
}
