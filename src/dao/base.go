package dao

import (
	"context"
	"fmt"

	"xiaozhi-server-go/src/configs/database"
	"xiaozhi-server-go/src/core/utils"

	"gorm.io/gorm"
)

// BaseDAO 基础DAO，提供通用的数据库操作方法
type BaseDAO struct {
	db *gorm.DB
}

// NewBaseDAO 创建基础DAO实例
func NewBaseDAO() *BaseDAO {
	return &BaseDAO{
		db: database.DB,
	}
}

// QueryOptions 查询选项配置
type QueryOptions struct {
	Table     string                   // 表名
	Where     map[string]interface{}   // WHERE条件，key为字段名，value为值
	WhereIn   map[string]interface{}   // WHERE IN条件
	WhereLike map[string]string        // WHERE LIKE条件
	Select    []string                 // 选择的字段，空则选择全部
	OrderBy   []string                 // 排序字段，例如 ["created_at DESC", "id ASC"]
	GroupBy   []string                 // 分组字段
	Having    string                   // HAVING条件
	Limit     int                      // 限制条数
	Offset    int                      // 偏移量
	Joins     []string                 // JOIN语句
	Preload   []string                 // 预加载关联
	Distinct  bool                     // 是否去重
	OrWhere   []map[string]interface{} // OR条件组
}

// Query 通用查询方法
func (dao *BaseDAO) Query(ctx context.Context, result interface{}, options QueryOptions) error {
	query := dao.db

	// 设置表名
	if options.Table != "" {
		query = query.Table(options.Table)
	}

	// 设置SELECT字段
	if len(options.Select) > 0 {
		query = query.Select(options.Select)
	}

	// 设置DISTINCT
	if options.Distinct {
		query = query.Distinct()
	}

	// 设置WHERE条件
	for field, value := range options.Where {
		query = query.Where(fmt.Sprintf("%s = ?", field), value)
	}

	// 设置WHERE IN条件
	for field, values := range options.WhereIn {
		query = query.Where(fmt.Sprintf("%s IN ?", field), values)
	}

	// 设置WHERE LIKE条件
	for field, pattern := range options.WhereLike {
		query = query.Where(fmt.Sprintf("%s LIKE ?", field), pattern)
	}

	// 设置OR WHERE条件
	for _, orCondition := range options.OrWhere {
		orQuery := dao.db.Where("1=0") // 初始化一个假的条件
		for field, value := range orCondition {
			orQuery = orQuery.Or(fmt.Sprintf("%s = ?", field), value)
		}
		query = query.Where(orQuery)
	}

	// 设置JOIN
	for _, join := range options.Joins {
		query = query.Joins(join)
	}

	// 设置预加载
	for _, preload := range options.Preload {
		query = query.Preload(preload)
	}

	// 设置GROUP BY
	if len(options.GroupBy) > 0 {
		for _, group := range options.GroupBy {
			query = query.Group(group)
		}
	}

	// 设置HAVING
	if options.Having != "" {
		query = query.Having(options.Having)
	}

	// 设置ORDER BY
	if len(options.OrderBy) > 0 {
		for _, order := range options.OrderBy {
			query = query.Order(order)
		}
	}

	// 设置LIMIT
	if options.Limit > 0 {
		query = query.Limit(options.Limit)
	}

	// 设置OFFSET
	if options.Offset > 0 {
		query = query.Offset(options.Offset)
	}

	// 执行查询
	if err := query.Find(result).Error; err != nil {
		utils.Error(ctx, fmt.Sprintf("查询数据失败: %v", err))
		return fmt.Errorf("查询数据失败: %w", err)
	}

	return nil
}

// Count 计数查询
func (dao *BaseDAO) Count(ctx context.Context, options QueryOptions) (int64, error) {
	query := dao.db

	// 设置表名
	if options.Table != "" {
		query = query.Table(options.Table)
	}

	// 设置WHERE条件
	for field, value := range options.Where {
		query = query.Where(fmt.Sprintf("%s = ?", field), value)
	}

	// 设置WHERE IN条件
	for field, values := range options.WhereIn {
		query = query.Where(fmt.Sprintf("%s IN ?", field), values)
	}

	// 设置WHERE LIKE条件
	for field, pattern := range options.WhereLike {
		query = query.Where(fmt.Sprintf("%s LIKE ?", field), pattern)
	}

	// 设置OR WHERE条件
	for _, orCondition := range options.OrWhere {
		orQuery := dao.db.Where("1=0")
		for field, value := range orCondition {
			orQuery = orQuery.Or(fmt.Sprintf("%s = ?", field), value)
		}
		query = query.Where(orQuery)
	}

	// 设置JOIN
	for _, join := range options.Joins {
		query = query.Joins(join)
	}

	var count int64
	if err := query.Count(&count).Error; err != nil {
		utils.Error(ctx, fmt.Sprintf("计数查询失败: %v", err))
		return 0, fmt.Errorf("计数查询失败: %w", err)
	}

	return count, nil
}

// Create 创建单条记录
func (dao *BaseDAO) Create(ctx context.Context, data interface{}) error {
	if err := dao.db.Create(data).Error; err != nil {
		utils.Error(ctx, fmt.Sprintf("创建记录失败: %v", err))
		return fmt.Errorf("创建记录失败: %w", err)
	}
	return nil
}

// CreateInBatches 批量创建记录
func (dao *BaseDAO) CreateInBatches(ctx context.Context, data interface{}, batchSize int) error {
	if err := dao.db.CreateInBatches(data, batchSize).Error; err != nil {
		utils.Error(ctx, fmt.Sprintf("批量创建记录失败: %v", err))
		return fmt.Errorf("批量创建记录失败: %w", err)
	}
	return nil
}

// Update 更新记录
func (dao *BaseDAO) Update(ctx context.Context, table string, where map[string]interface{}, updates map[string]interface{}) error {
	query := dao.db.Table(table)

	// 设置WHERE条件
	for field, value := range where {
		query = query.Where(fmt.Sprintf("%s = ?", field), value)
	}

	if err := query.Updates(updates).Error; err != nil {
		utils.Error(ctx, fmt.Sprintf("更新记录失败: %v", err))
		return fmt.Errorf("更新记录失败: %w", err)
	}

	return nil
}

// Delete 删除记录
func (dao *BaseDAO) Delete(ctx context.Context, table string, where map[string]interface{}) error {
	query := dao.db.Table(table)

	// 设置WHERE条件
	for field, value := range where {
		query = query.Where(fmt.Sprintf("%s = ?", field), value)
	}

	if err := query.Delete(nil).Error; err != nil {
		utils.Error(ctx, fmt.Sprintf("删除记录失败: %v", err))
		return fmt.Errorf("删除记录失败: %w", err)
	}

	return nil
}

// First 查询单条记录
func (dao *BaseDAO) First(ctx context.Context, result interface{}, options QueryOptions) error {
	query := dao.db

	// 设置表名
	if options.Table != "" {
		query = query.Table(options.Table)
	}

	// 设置SELECT字段
	if len(options.Select) > 0 {
		query = query.Select(options.Select)
	}

	// 设置WHERE条件
	for field, value := range options.Where {
		query = query.Where(fmt.Sprintf("%s = ?", field), value)
	}

	// 设置WHERE IN条件
	for field, values := range options.WhereIn {
		query = query.Where(fmt.Sprintf("%s IN ?", field), values)
	}

	// 设置WHERE LIKE条件
	for field, pattern := range options.WhereLike {
		query = query.Where(fmt.Sprintf("%s LIKE ?", field), pattern)
	}

	// 设置JOIN
	for _, join := range options.Joins {
		query = query.Joins(join)
	}

	// 设置预加载
	for _, preload := range options.Preload {
		query = query.Preload(preload)
	}

	// 设置ORDER BY
	if len(options.OrderBy) > 0 {
		for _, order := range options.OrderBy {
			query = query.Order(order)
		}
	}

	// 执行查询
	if err := query.First(result).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil // 记录不存在不算错误
		}
		utils.Error(ctx, fmt.Sprintf("查询单条记录失败: %v", err))
		return fmt.Errorf("查询单条记录失败: %w", err)
	}

	return nil
}

// Exists 检查记录是否存在
func (dao *BaseDAO) Exists(ctx context.Context, options QueryOptions) (bool, error) {
	count, err := dao.Count(ctx, options)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// Transaction 事务操作
func (dao *BaseDAO) Transaction(ctx context.Context, fn func(*gorm.DB) error) error {
	return dao.db.Transaction(fn)
}

// GetDB 获取数据库连接（供复杂查询使用）
func (dao *BaseDAO) GetDB() *gorm.DB {
	return dao.db
}
