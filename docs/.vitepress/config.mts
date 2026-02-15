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
            { text: 'CLI 命令参考', link: '/agix/cli' },
            { text: '配置文件', link: '/agix/config' },
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
