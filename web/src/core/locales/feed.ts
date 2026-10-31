// Subscribe page copy. Kept in its own domain file (rather than folded into
// `site`, which is the admin settings form) so the reader-facing wording for
// "what is a feed" lives next to the only screen that shows it.

export type FeedTexts = {
  feed: {
    title: string
    intro: string
    address: string
    copy: string
    copied: string
    open: string
    readersTitle: string
    readersHint: string
    noteTitle: string
    note: string
    footerLink: string
    readLocked: string
  }
}

export const feedZh: FeedTexts = {
  feed: {
    title: '订阅本站',
    intro: '本站提供 RSS 订阅源。订阅之后，新文章会自动出现在你的阅读器里，不用反复回来看有没有更新。',
    address: '订阅地址',
    copy: '复制地址',
    copied: '已复制',
    open: '直接打开订阅源',
    readersTitle: '用什么订阅',
    readersHint: '把上面的地址粘贴到任意 RSS 阅读器即可，例如 Feedly、Inoreader、NetNewsWire、Follow，或者浏览器上的 RSS 扩展。也可以直接把它加进支持订阅的笔记工具。',
    noteTitle: '点开为什么是一堆代码',
    note: 'RSS 是一份给阅读器读的 XML，不是给人看的网页。直接点开会看到源码，这是正常的——把它交给阅读器，它才会变成文章列表。',
    footerLink: '订阅',
    readLocked: '本站当前关闭了匿名阅读，订阅源暂时不可用。',
  },
}

export const feedEn: FeedTexts = {
  feed: {
    title: 'Subscribe',
    intro: 'This site publishes an RSS feed. Subscribe once and new posts show up in your reader — no need to keep checking back.',
    address: 'Feed address',
    copy: 'Copy address',
    copied: 'Copied',
    open: 'Open the raw feed',
    readersTitle: 'Where to read it',
    readersHint: 'Paste the address above into any RSS reader — Feedly, Inoreader, NetNewsWire, Follow, or a browser extension. Note-taking tools that accept feeds work too.',
    noteTitle: 'Why it looks like code',
    note: 'RSS is an XML document written for a reader, not a web page. Opening it directly shows the source, which is expected — hand it to a reader and it becomes a list of articles.',
    footerLink: 'Subscribe',
    readLocked: 'Anonymous reading is currently off, so the feed is unavailable.',
  },
}
