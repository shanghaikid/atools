import { defineConfig } from 'vitepress'

export default defineConfig({
  title: 'Agent Platform Tools',
  description: 'AI Agent 平台工具集文档',
  lang: 'zh-CN',
  cleanUrls: true,

  themeConfig: {
    nav: [
      { text: '首页', link: '/' },
      { text: 'agix', link: '/agix/' },
      { text: 'ainit', link: '/ainit/' },
      { text: 'worldtime', link: '/worldtime/' },
    ],

    sidebar: {
      '/agix/': [
        {
          text: 'agix',
          items: [
            { text: '简介', link: '/agix/' },
            { text: '快速开始', link: '/agix/quickstart' },
            { text: '核心功能', link: '/agix/features' },
            {
              text: 'CLI 命令参考',
              collapsed: false,
              items: [
                { text: '概览', link: '/agix/cli/' },
                { text: 'init · start', link: '/agix/cli/init-start' },
                { text: 'stats · logs · export', link: '/agix/cli/stats-logs' },
                { text: 'budget', link: '/agix/cli/budget' },
                { text: 'tools · bundle', link: '/agix/cli/tools-bundle' },
                { text: 'doctor', link: '/agix/cli/doctor' },
                { text: 'trace', link: '/agix/cli/trace' },
                { text: 'experiment', link: '/agix/cli/experiment' },
                { text: 'audit · session · webhook', link: '/agix/cli/advanced' },
              ],
            },
            { text: '配置文件', link: '/agix/config' },
            { text: 'HTTP API 参考', link: '/agix/api-reference' },
          ],
        },
        {
          text: '使用指南',
          items: [
            { text: '指南导航', link: '/agix/guides/' },
            { text: '💰 成本追踪与预算', link: '/agix/guides/cost-tracking' },
            { text: '🧠 智能优化', link: '/agix/guides/intelligence-optimization' },
            { text: '🔒 安全与控制', link: '/agix/guides/safety-control' },
            { text: '📊 可观测性', link: '/agix/guides/observability' },
            { text: '🚀 可靠性与扩展', link: '/agix/guides/reliability-scale' },
            {
              text: '⚙️ 高级功能',
              collapsed: false,
              items: [
                { text: '概览', link: '/agix/guides/advanced/' },
                { text: '系统提示词注入', link: '/agix/guides/advanced/prompt-injection' },
                { text: 'MCP 工具包', link: '/agix/guides/advanced/mcp-bundle' },
                { text: 'PostgreSQL 后端', link: '/agix/guides/advanced/postgres' },
                { text: 'DeepSeek 提供商', link: '/agix/guides/advanced/deepseek' },
              ],
            },
            { text: '🔧 故障排查与FAQ', link: '/agix/guides/troubleshooting' },
          ],
        },
      ],
      '/ainit/': [
        {
          text: 'ainit',
          items: [
            { text: '简介', link: '/ainit/' },
            { text: '安装与使用', link: '/ainit/usage' },
            { text: 'Agent 模板', link: '/ainit/agents' },
          ],
        },
      ],
      '/worldtime/': [
        {
          text: 'worldtime',
          items: [
            { text: '简介', link: '/worldtime/' },
          ],
        },
      ],
    },

    socialLinks: [
      { icon: 'github', link: 'https://github.com/ryjiang/agent-platform' },
    ],

    outline: { level: [2, 3], label: '目录' },
    lastUpdated: { text: '最后更新' },
    docFooter: { prev: '上一篇', next: '下一篇' },
  },
})
