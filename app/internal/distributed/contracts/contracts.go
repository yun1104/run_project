package contracts

type BaseResponse struct {
	Code    int32  `json:"code"`
	Message string `json:"message"`
}

type UserIDRequest struct {
	UserID int64 `json:"user_id"`
}

type RegisterRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
	Phone    string `json:"phone"`
}

type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type LoginResponse struct {
	Code    int32  `json:"code"`
	Message string `json:"message"`
	Token   string `json:"token"`
	UserID  int64  `json:"user_id"`
}

type ValidateTokenRequest struct {
	Token string `json:"token"`
}

type ValidateTokenResponse struct {
	Code    int32  `json:"code"`
	Message string `json:"message"`
	UserID  int64  `json:"user_id"`
	Valid   bool   `json:"valid"`
}

type MeResponse struct {
	Code    int32       `json:"code"`
	Message string      `json:"message"`
	Data    MeResponseData `json:"data"`
}

type MeResponseData struct {
	UserID             int64  `json:"user_id"`
	Username           string `json:"username"`
	LocationPermission string `json:"location_permission"`
}

type UpdateLocationPermissionRequest struct {
	UserID             int64  `json:"user_id"`
	LocationPermission string `json:"location_permission"`
}

type LocationPermissionResponse struct {
	Code               int32  `json:"code"`
	Message            string `json:"message"`
	LocationPermission string `json:"location_permission"`
}

type UserPreference struct {
	UserID       int64    `json:"user_id"`
	Categories   []string `json:"categories"`
	PriceRange   string   `json:"price_range"`
	Tastes       []string `json:"tastes"`
	Merchants    []int64  `json:"merchants"`
	DishKeywords []string `json:"dish_keywords"`
	AvoidFoods   []string `json:"avoid_foods"`
	OrderTimes   []int32  `json:"order_times"`
}

type PreferenceResponse struct {
	Code    int32                `json:"code"`
	Message string               `json:"message"`
	Data    PreferenceResponseData `json:"data"`
}

type PreferenceResponseData struct {
	HasPreference bool            `json:"has_preference"`
	Preference    *UserPreference `json:"preference"`
}

type UpdatePreferenceRequest struct {
	UserID       int64    `json:"user_id"`
	Categories   []string `json:"categories"`
	PriceRange   string   `json:"price_range"`
	Tastes       []string `json:"tastes"`
	Merchants    []int64  `json:"merchants"`
	DishKeywords []string `json:"dish_keywords"`
	AvoidFoods   []string `json:"avoid_foods"`
	OrderTimes   []int32  `json:"order_times"`
}

type OrderItem struct {
	Name     string  `json:"name"`
	Quantity int32   `json:"quantity"`
	Price    float64 `json:"price"`
}

type Order struct {
	OrderID         int64       `json:"order_id"`
	UserID          int64       `json:"user_id"`
	MerchantID      int64       `json:"merchant_id"`
	MerchantName    string      `json:"merchant_name"`
	Items           []OrderItem `json:"items"`
	DeliveryAddress string      `json:"delivery_address"`
	Remark          string      `json:"remark"`
	Amount          float64     `json:"amount"`
	Status          string      `json:"status"`
}

type CreateOrderRequest struct {
	UserID          int64       `json:"user_id"`
	MerchantID      int64       `json:"merchant_id"`
	MerchantName    string      `json:"merchant_name"`
	Items           []OrderItem `json:"items"`
	DeliveryAddress string      `json:"delivery_address"`
	Remark          string      `json:"remark"`
}

type CreateOrderResponse struct {
	Code    int32  `json:"code"`
	Message string `json:"message"`
	OrderID int64  `json:"order_id"`
}

type ListOrdersRequest struct {
	UserID int64 `json:"user_id"`
}

type ListOrdersResponse struct {
	Code    int32   `json:"code"`
	Message string  `json:"message"`
	Orders  []Order `json:"orders"`
}

type RecommendRequest struct {
	UserID      int64  `json:"user_id"`
	Requirement string `json:"requirement"`
	Location    string `json:"location"`
}

type Merchant struct {
	ID           int64    `json:"id"`
	Name         string   `json:"name"`
	Category     string   `json:"category"`
	Rating       float64  `json:"rating"`
	AvgPrice     float64  `json:"avg_price"`
	Distance     string   `json:"distance"`
	DeliveryTime int32    `json:"delivery_time"`
	Tags         []string `json:"tags"`
	Reason       string   `json:"reason"`
	Score        float64  `json:"score"`
}

type RecommendResponse struct {
	Code      int32      `json:"code"`
	Message   string     `json:"message"`
	Merchants []Merchant `json:"merchants"`
}
