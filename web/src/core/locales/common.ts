// Shared labels.

export type CommonTexts = {
  common: {
    loading: string
    count: string
    // Rendered whenever the server answers 429. Showing the raw English body
    // ("too many requests") in a Chinese UI is the kind of mixed language that
    // makes a site feel unfinished.
    tooManyRequests: string
    // Same reasoning for the other mechanical failures: the server's text is
    // English and says nothing the user can act on.
    tooLarge: string
    serverError: string
    unavailable: string
  }
}

export const commonZh: CommonTexts = {
  common: {
    loading: '加载中…',
    count: '{from}–{to} / {total}',
    tooManyRequests: '操作太频繁，请稍后再试',
    tooLarge: '文件太大，超出允许的上限',
    serverError: '服务器出错了，请稍后重试',
    unavailable: '服务暂时不可用，请稍后重试',
  }
}

export const commonEn: CommonTexts = {
  common: {
    loading: 'Loading…',
    count: '{from}–{to} / {total}',
    tooManyRequests: 'Too many requests — please wait a moment',
    tooLarge: 'That file is too large',
    serverError: 'Something went wrong on the server — please try again',
    unavailable: 'The service is temporarily unavailable — please try again',
  }
}
