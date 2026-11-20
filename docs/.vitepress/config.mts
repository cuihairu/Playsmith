import { defineConfig } from "vitepress";
import { redirectSnippet } from "./redirect.mjs";

// published as a GitHub project page: cuihairu.github.io/Playsmith/
// The redirect snippet interpolates the same value, so it cannot drift from
// the deployed base.
const base = "/Playsmith/";

export default defineConfig({
  base,
  // First visit only: a zh* browser lands on an English page once, then the
  // manual locale switch is the single source of truth (see redirect.mjs).
  head: [["script", {}, redirectSnippet(base)]],
  // The consistency-audit ledger is a working record (round counts, commit
  // hashes, findings) rather than navigable site content.
  srcExclude: ["审计-文档一致性.md"],
  // architecture.md / glossary.md cite repo files as `../<path>:<line>` markdown
  // links. Those resolve on GitHub but are outside the docs src root, so the
  // site build must not treat them as internal pages.
  ignoreDeadLinks: [/^(\.\/)?\.\.\//],
  locales: {
    root: {
      label: "English",
      lang: "en-US",
      title: "Playsmith",
      description:
        "ERC-1155-style game assets on Hyperledger Fabric: mint, transfer, market, indexer",
      themeConfig: {
        nav: [
          { text: "Home", link: "/" },
          {
            text: "Docs",
            items: [
              { text: "Features", link: "/features" },
              { text: "Configuration", link: "/configuration" },
              { text: "Build", link: "/build" },
              { text: "Architecture", link: "/architecture" },
              { text: "Glossary", link: "/glossary" },
            ],
          },
        ],
        sidebar: [
          {
            text: "Docs",
            items: [
              { text: "Home", link: "/" },
              { text: "Features", link: "/features" },
              { text: "Configuration", link: "/configuration" },
              { text: "Build", link: "/build" },
              { text: "Architecture", link: "/architecture" },
              { text: "Glossary", link: "/glossary" },
            ],
          },
        ],
      },
    },
    zh: {
      label: "简体中文",
      lang: "zh-CN",
      title: "Playsmith",
      description: "Hyperledger Fabric 上的 ERC-1155 风格游戏资产：铸造、转移、市场、索引",
      themeConfig: {
        nav: [
          { text: "首页", link: "/zh/" },
          {
            text: "文档",
            items: [
              { text: "功能", link: "/zh/features" },
              { text: "配置", link: "/zh/configuration" },
              { text: "构建", link: "/zh/build" },
              { text: "架构（英文）", link: "/architecture" },
              { text: "术语表（英文）", link: "/glossary" },
            ],
          },
        ],
        sidebar: [
          {
            text: "文档",
            items: [
              { text: "首页", link: "/zh/" },
              { text: "功能", link: "/zh/features" },
              { text: "配置", link: "/zh/configuration" },
              { text: "构建", link: "/zh/build" },
              { text: "架构（英文）", link: "/architecture" },
              { text: "术语表（英文）", link: "/glossary" },
            ],
          },
        ],
      },
    },
  },
  themeConfig: {
    socialLinks: [
      { icon: "github", link: "https://github.com/cuihairu/Playsmith" },
    ],
  },
});
