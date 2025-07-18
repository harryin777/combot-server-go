package router

func ActiveRouter(router *Router) {
	// 注册激活相关路由
	router.Group("/active", func(router *Router) {
		router.POST("/register", router.Active.Register)
		router.POST("/login", router.Active.Login)
		router.POST("/logout", router.Active.Logout)
		router.GET("/info", router.Active.Info)
	})
}
