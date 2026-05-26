package interceptor

//работать будет так:
//стартуется playwright браузер  -> парсятся валидные куки -> эти куки передаются в playwright instanse -> переход по loginURL -> ввод в чат двух сообщений -> перехват responses на второе сообщение

const loginURL = "https://accounts.x.ai/sign-in?redirect=grok-com&return_to=/?q=%26reasoningMode=none%26voice=false&email=true"

type Interceptor struct {
}
