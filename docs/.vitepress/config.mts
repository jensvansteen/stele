import { defineConfig } from "vitepress";

export default defineConfig({
  base: "/stele/",
  lang: "en-US",
  title: "Stele",
  description: "Deterministic evidence for plain-English software specifications.",
  cleanUrls: true,
  lastUpdated: true,
  head: [
    ["link", { rel: "icon", type: "image/svg+xml", href: "/stele/stele.svg" }],
    ["meta", { name: "theme-color", content: "#0d1117" }],
  ],
  themeConfig: {
    logo: "/stele.svg",
    siteTitle: "Stele",
    search: { provider: "local" },
    nav: [
      { text: "Guide", link: "/guide/getting-started" },
      { text: "Concepts", link: "/concepts/openspec-and-stele" },
      { text: "Reference", link: "/reference/cli" },
      { text: "Roadmap", link: "/roadmap/dashboard" },
    ],
    sidebar: [
      {
        text: "Guide",
        items: [
          { text: "Getting started", link: "/guide/getting-started" },
          { text: "Build a verified change", link: "/guide/verified-change" },
          { text: "Use a local package", link: "/guide/local-package" },
        ],
      },
      {
        text: "Concepts",
        items: [
          { text: "OpenSpec and Stele", link: "/concepts/openspec-and-stele" },
          { text: "IDs and anchors", link: "/concepts/ids-and-anchors" },
          { text: "Plan test levels", link: "/concepts/test-levels" },
          { text: "Deterministic evidence", link: "/concepts/deterministic-evidence" },
        ],
      },
      {
        text: "Reference",
        items: [
          { text: "CLI", link: "/reference/cli" },
          { text: "Package architecture", link: "/reference/architecture" },
          { text: "Performance", link: "/reference/performance" },
          { text: "Examples", link: "/reference/examples" },
        ],
      },
      {
        text: "Direction",
        items: [{ text: "Dashboard roadmap", link: "/roadmap/dashboard" }],
      },
    ],
    socialLinks: [{ icon: "github", link: "https://github.com/jensvansteen/stele" }],
    editLink: {
      pattern: "https://github.com/jensvansteen/stele/edit/main/docs/:path",
      text: "Edit this page",
    },
    footer: {
      message: "OpenSpec owns behavior. Stele verifies the evidence graph.",
    },
    outline: { level: [2, 3], label: "On this page" },
  },
});
