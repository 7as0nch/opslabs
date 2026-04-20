package base

import (
	"context"
	"strconv"

	pb "github.com/example/aichat/backend/api/base"
	"github.com/example/aichat/backend/internal/biz/base"
	"github.com/example/aichat/backend/models"
	"github.com/example/aichat/backend/models/generator/model"
	"google.golang.org/protobuf/types/known/emptypb"
)

type SystemService struct {
	pb.UnimplementedSystemServer
	menu      *base.SysMenuUseCase
	dictType  *base.DictTypeUseCase
	dictData  *base.DictDataUseCase
}

func NewSystemService(menu *base.SysMenuUseCase, dictType *base.DictTypeUseCase, dictData *base.DictDataUseCase) *SystemService {
	return &SystemService{
		menu:     menu,
		dictType: dictType,
		dictData: dictData,
	}
}

func (s *SystemService) Menu(ctx context.Context, req *emptypb.Empty) (*pb.MenuReply, error) {
	menus, err := s.menu.GetRouter(ctx)
	if err != nil {
		return nil, err
	}
	var pbMenus = make([]*pb.Menu, 0, len(menus))
	for _, m := range menus {
		pbMenus = append(pbMenus, &pb.Menu{
			Id:        m.ID,
			ParentID:  m.ParentID,
			Name:      m.Name,
			Path:      m.Path,
			Component: m.Component,
			// Sort:      m.Sort,
			Hidden:     m.Hidden,
			AlwaysShow: m.AlwaysShow,
			Redirect:   m.Redirect,
			Meta: &pb.MenuMeta{
				Title:   m.Meta.Title,
				Icon:    m.Meta.Icon,
				NoCache: m.Meta.NoCache,
			},
		})
	}
	return &pb.MenuReply{
		Menu: pbMenusToTree(pbMenus, 0),
	}, nil
}

func (s *SystemService) AllMenu(ctx context.Context, req *emptypb.Empty) (*pb.AllMenuReply, error) {
	menus, err := s.menu.GetAll(ctx)
	if err != nil {
		return nil, err
	}
	var pbMenus = make([]*pb.MenuItem, 0, len(menus))
	for _, m := range menus {
		pbMenus = append(pbMenus, &pb.MenuItem{
			CreateBy:   "",
			CreateTime: m.CreatedAt.String(),
			UpdateBy:   "",
			UpdateTime: m.UpdatedAt.String(),
			Remark:     "",
			MenuId:     m.ID,
			MenuName:   m.Meta.Title,
			ParentId:   m.ParentID,
			ParentName: "",
			OrderNum:   int32(m.Sort),
			Path:       m.Path,
			Component:  m.Component,
			Query:      "",
			IsFrame:    "1",
			IsCache:    func() string {
				if m.Meta.NoCache {
					return "1"
				}
				return "0"
			}(),
			MenuType:   m.Type.String(),
			Visible:    "1",
			Status:     m.Status.String(),
			Perms:      m.PermsCode,
			Icon:       m.Meta.Icon,
		})
	}
	return &pb.AllMenuReply{
		Menus: pbMenus,
	}, nil
}

// pbMenus to tree
func pbMenusToTree(menus []*pb.Menu, parentID int64) []*pb.Menu {
	var tree []*pb.Menu
	for _, menu := range menus {
		if menu.ParentID == parentID {
			// 递归查找子菜单
			menu.Children = pbMenusToTree(menus, menu.Id)
			tree = append(tree, menu)
		}
	}
	return tree
}

// add
func (s *SystemService) AddSysMenu(ctx context.Context, req *pb.AddSysMenuRequest) (*emptypb.Empty, error) {
	t := &model.SysMenu{
		Name:      req.Component,
		Component: req.Component,
		Path:      req.Path,
		// Query:     req.Query,
		// Redirect:  req.Redirect,
		ParentID: req.ParentId,
		Sort:     int(req.OrderNum),
		Type:     model.ToMenuType(req.MenuType),
		Hidden:   req.Visible == "2",
		// AlwaysShow: req.AlwaysShow,
		Meta: &model.Meta{
			Title:   req.MenuName,
			Icon:    req.Icon,
			NoCache: req.IsCache == "1",
		},
		PermsCode: req.Perms,
		Remark:    req.Remark,
	}
	t.New()
	err := s.menu.Add(ctx, t)
	if err != nil {
		return nil, err
	}
	return &emptypb.Empty{}, nil
}

// UpdateSysMenu
func (s *SystemService) UpdateSysMenu(ctx context.Context, req *pb.AddSysMenuRequest) (*emptypb.Empty, error) {
	t := &model.SysMenu{
		Name:      req.Component,
		Component: req.Component,
		Path:      req.Path,
		// Query:     req.Query,
		// Redirect:  req.Redirect,
		ParentID: req.ParentId,
		Sort:     int(req.OrderNum),
		Type:     model.ToMenuType(req.MenuType),
		Hidden:   req.Visible == "2",
		// AlwaysShow: req.AlwaysShow,
		Meta: &model.Meta{
			Title:   req.MenuName,
			Icon:    req.Icon,
			NoCache: req.IsCache == "1",
		},
		PermsCode: req.Perms,
		Remark:    req.Remark,
	}
	t.ID = req.MenuId
	err := s.menu.Update(ctx, t)
	if err != nil {
		return nil, err
	}
	return &emptypb.Empty{}, nil
}

// DeleteSysMenu
func (s *SystemService) DeleteSysMenu(ctx context.Context, req *pb.DeleteSysMenuRequest) (*emptypb.Empty, error) {
	err := s.menu.Delete(ctx, req.Id)
	if err != nil {
		return nil, err
	}
	return &emptypb.Empty{}, nil
}

// GetSysMenu
func (s *SystemService) GetSysMenu(ctx context.Context, req *pb.GetSysMenuRequest) (*pb.MenuItem, error) {
	m, err := s.menu.Get(ctx, req.Id)
	if err != nil {
		return nil, err
	}
	return &pb.MenuItem{
			CreateBy:   "",
			CreateTime: m.CreatedAt.String(),
			UpdateBy:   "",
			UpdateTime: m.UpdatedAt.String(),
			Remark:     "",
			MenuId:     m.ID,
			MenuName:   m.Meta.Title,
			ParentId:   m.ParentID,
			ParentName: "",
			OrderNum:   int32(m.Sort),
			Path:       m.Path,
			Component:  m.Component,
			Query:      "",
			IsFrame:    "1",
			IsCache:    "1",
			MenuType:   m.Type.String(),
			Visible:    "1",
			Status:     "1",
			Perms:      m.PermsCode,
			Icon:       m.Meta.Icon,
		}, nil
}

// ==================== 字典类型管理接口 ====================

// DictTypeList 获取字典类型列表
func (s *SystemService) DictTypeList(ctx context.Context, req *pb.DictTypeListRequest) (*pb.DictTypeListReply, error) {
	types, total, err := s.dictType.DictTypeList(ctx, req.PageNum, req.PageSize, req.DictType, req.DictName)
	if err != nil {
		return nil, err
	}
	
	var pbTypes = make([]*pb.DictType, 0, len(types))
	for _, t := range types {
		pbTypes = append(pbTypes, &pb.DictType{
			DictId:    t.ID,
			DictName:  t.DictName,
			DictType:  t.DictType,
			Status:    t.Status.String(),
			Remark:    t.Remark,
			CreateTime: t.CreatedAt.Unix(),
		})
	}
	
	return &pb.DictTypeListReply{
		List:  pbTypes,
		Total: int32(total),
	}, nil
}

// DictTypeById 根据ID获取字典类型
func (s *SystemService) DictTypeById(ctx context.Context, req *pb.DictRequest) (*pb.DictType, error) {
	typ, err := s.dictType.DictTypeById(ctx, req.Id)
	if err != nil {
		return nil, err
	}
	
	return &pb.DictType{
		DictId:    typ.ID,
		DictName:  typ.DictName,
		DictType:  typ.DictType,
		Status:    typ.Status.String(),
		Remark:    typ.Remark,
		CreateTime: typ.CreatedAt.Unix(),
	}, nil
}

// AddDictType 添加字典类型
func (s *SystemService) AddDictType(ctx context.Context, req *pb.DictType) (*emptypb.Empty, error) {
	typ := &model.SysDictType{
		DictName: req.DictName,
		DictType: req.DictType,
		Remark:   req.Remark,
		Status:   models.ToStatus(req.Status),
	}
	typ.New()
	
	err := s.dictType.AddDictType(ctx, typ)
	if err != nil {
		return nil, err
	}
	return &emptypb.Empty{}, nil
}

// UpdateDictType 更新字典类型
func (s *SystemService) UpdateDictType(ctx context.Context, req *pb.DictType) (*emptypb.Empty, error) {
	typ := &model.SysDictType{
		DictName: req.DictName,
		DictType: req.DictType,
		Remark:   req.Remark,
		Status:   models.ToStatus(req.Status),
	}
	typ.ID = req.DictId
	err := s.dictType.UpdateDictType(ctx, typ)
	if err != nil {
		return nil, err
	}
	return &emptypb.Empty{}, nil
}

// DeleteDictType 删除字典类型
func (s *SystemService) DeleteDictType(ctx context.Context, req *pb.DictRequest) (*emptypb.Empty, error) {
	err := s.dictType.DeleteDictType(ctx, req.Id)
	if err != nil {
		return nil, err
	}
	return &emptypb.Empty{}, nil
}

// ==================== 字典数据管理接口 ====================

// DictDataList 获取字典数据列表
func (s *SystemService) DictDataList(ctx context.Context, req *pb.DictDataListRequest) (*pb.DictDataListReply, error) {
	datas, total, err := s.dictData.DictDataList(ctx, req.PageNum, req.PageSize, req.DictLabel, req.DictType)
	if err != nil {
		return nil, err
	}
	
	var pbDatas = make([]*pb.DictData, 0, len(datas))
	for _, d := range datas {
			isDefaultStr := "N"
			if d.IsDefault {
				isDefaultStr = "Y"
			}
			pbDatas = append(pbDatas, &pb.DictData{
				DictCode:   d.DictCode,
				DictSort:   int32(d.DictSort),
				DictLabel:  d.DictLabel,
				DictValue:  d.DictValue,
				DictType:   d.DictType,
				ListClass:  d.ListClass,
				IsDefault:  isDefaultStr,
				Status:     d.Status.String(),
				CreateTime: d.CreatedAt.String(),
			})
		}
	
	return &pb.DictDataListReply{
		List:  pbDatas,
		Total: int32(total),
	}, nil
}

// DictDataListByType 根据类型获取字典数据列表
func (s *SystemService) DictDataListByType(ctx context.Context, req *pb.DictDataListByTypeRequest) (*pb.DictDataListReply, error) {
	datas, err := s.dictData.DictDataListByType(ctx, req.DictType)
	if err != nil {
		return nil, err
	}
	
	var pbDatas = make([]*pb.DictData, 0, len(datas))
	for _, d := range datas {
			isDefaultStr := "N"
			if d.IsDefault {
				isDefaultStr = "Y"
			}
			pbDatas = append(pbDatas, &pb.DictData{
				DictCode:   d.DictCode,
				DictSort:   int32(d.DictSort),
				DictLabel:  d.DictLabel,
				DictValue:  d.DictValue,
				DictType:   d.DictType,
				ListClass:  d.ListClass,
				IsDefault:  isDefaultStr,
				Status:     d.Status.String(),
				CreateTime: d.CreatedAt.String(),
			})
		}
	
	return &pb.DictDataListReply{
		List:  pbDatas,
		Total: int32(len(pbDatas)),
	}, nil
}

// DictDataById 根据ID获取字典数据
func (s *SystemService) DictDataById(ctx context.Context, req *pb.DictRequest) (*pb.DictData, error) {
	d, err := s.dictData.DictDataById(ctx, req.Id)
	if err != nil {
		return nil, err
	}
	
	isDefaultStr := "N"
	if d.IsDefault {
		isDefaultStr = "Y"
	}
	
	return &pb.DictData{
		DictCode:   d.DictCode,
		DictSort:   int32(d.DictSort),
		DictLabel:  d.DictLabel,
		DictValue:  d.DictValue,
		DictType:   d.DictType,
		ListClass:  d.ListClass,
		IsDefault:  isDefaultStr,
		Status:     d.Status.String(),
		CreateTime: d.CreatedAt.String(),
	}, nil
}

// AddDictData 添加字典数据
func (s *SystemService) AddDictData(ctx context.Context, req *pb.DictData) (*emptypb.Empty, error) {
	d := &model.SysDict{
		DictCode:  req.DictCode,
		DictSort:  int(req.DictSort),
		DictLabel: req.DictLabel,
		DictValue: req.DictValue,
		DictType:  req.DictType,
		ListClass: req.ListClass,
		Status:    models.ToStatus(req.Status),
	}
	d.New()
	d.DictCode = strconv.FormatInt(d.ID, 10)
	err := s.dictData.AddDictData(ctx, d)
	if err != nil {
		return nil, err
	}
	return &emptypb.Empty{}, nil
}

// UpdateDictData 更新字典数据
func (s *SystemService) UpdateDictData(ctx context.Context, req *pb.DictData) (*emptypb.Empty, error) {
	d := &model.SysDict{
		DictCode:  req.DictCode,
		DictSort:  int(req.DictSort),
		DictLabel: req.DictLabel,
		DictValue: req.DictValue,
		DictType:  req.DictType,
		ListClass: req.ListClass,
		Status:    models.ToStatus(req.Status),
	}
	d.ID, _ = strconv.ParseInt(req.DictCode, 10, 64)
	err := s.dictData.UpdateDictData(ctx, d)
	if err != nil {
		return nil, err
	}
	return &emptypb.Empty{}, nil
}

// DeleteDictData 删除字典数据
func (s *SystemService) DeleteDictData(ctx context.Context, req *pb.DictRequest) (*emptypb.Empty, error) {
	err := s.dictData.DeleteDictData(ctx, req.Id)
	if err != nil {
		return nil, err
	}
	return &emptypb.Empty{}, nil
}