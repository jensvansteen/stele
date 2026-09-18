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
    ],
    sidebar: [
      {
        text: "Guide",
        items: [
          { text: "Getting started", link: "/guide/getting-started" },
          { text: "Build the Todo feature", link: "/guide/build-todo" },
          { text: "Inspect the Todo example", link: "/guide/inspect-example" },
          { text: "Build a verified change", link: "/guide/verified-change" },
          { text: "Continuous integration", link: "/guide/continuous-integration" },
          { text: "Use Stele with OpenSpec", link: "/guide/openspec" },
          { text: "Use a local package", link: "/guide/local-package" },
          { text: "Editor integration", link: "/guide/editors" },
        ],
      },
      {
        text: "Concepts",
        items: [
          { text: "Product intent", link: "/intent/vision" },
          { text: "The Stele model", link: "/concepts/model" },
          { text: "Specification format", link: "/concepts/spec-format" },
          { text: "OpenSpec and Stele", link: "/concepts/openspec-and-stele" },
          { text: "IDs and anchors", link: "/concepts/ids-and-anchors" },
          { text: "Verification evidence", link: "/concepts/verification-evidence" },
          { text: "Deterministic evidence", link: "/concepts/deterministic-evidence" },
        ],
      },
      {
        text: "Reference",
        items: [
          { text: "CLI", link: "/reference/cli" },
          { text: "Link index", link: "/reference/link-index" },
          { text: "Package architecture", link: "/reference/architecture" },
          { text: "Versions", link: "/reference/versions" },
          { text: "Examples", link: "/reference/examples" },
        ],
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
