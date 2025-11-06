package com.myblogs.service;

import java.util.List;

import com.myblogs.entity.query.UserInfoQuery;
import com.myblogs.entity.po.UserInfo;
import com.myblogs.entity.vo.PaginationResultVO;
import com.myblogs.entity.vo.UserInfoVO;


/**
 * 用户信息表 业务接口
 */
public interface UserInfoService {

	/**
	 * 根据条件查询列表
	 */
	List<UserInfo> findListByParam(UserInfoQuery param);

	/**
	 * 根据条件查询列表
	 */
	Integer findCountByParam(UserInfoQuery param);

	/**
	 * 分页查询
	 */
	PaginationResultVO<UserInfo> findListByPage(UserInfoQuery param);

	/**
	 * 新增
	 */
	Integer add(UserInfo bean);

	/**
	 * 批量新增
	 */
	Integer addBatch(List<UserInfo> listBean);

	/**
	 * 批量新增/修改
	 */
	Integer addOrUpdateBatch(List<UserInfo> listBean);

	/**
	 * 多条件更新
	 */
	Integer updateByParam(UserInfo bean,UserInfoQuery param);

	/**
	 * 多条件删除
	 */
	Integer deleteByParam(UserInfoQuery param);

	/**
	 * 根据Id查询对象
	 */
	UserInfo getUserInfoById(Long id);

	UserInfo getUserInfoByEmail(String email);


	/**
	 * 根据Id修改
	 */
	Integer updateUserInfoById(UserInfo bean,Long id);


	/**
	 * 根据Id删除
	 */
	Integer deleteUserInfoById(Long id);


	UserInfoVO login(String email, String password);

	void register (String email, String nickname, String password);

}