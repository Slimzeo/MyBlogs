package com.myblogs.service.impl;

import java.util.Date;
import java.util.List;

import com.myblogs.entity.config.AppConfig;
import com.myblogs.entity.constants.Constants;
import com.myblogs.entity.dto.JwtUserInfoDto;
import com.myblogs.entity.enums.UserInfoStatusEnum;
import com.myblogs.entity.vo.UserInfoVO;
import com.myblogs.exception.BusinessException;
import com.myblogs.redis.RedisComponent;
import com.myblogs.utils.CopyTools;
import com.myblogs.security.JwtTools;
import com.myblogs.utils.HuTools;
import jakarta.annotation.Resource;

import org.springframework.stereotype.Service;

import com.myblogs.entity.enums.PageSize;
import com.myblogs.entity.query.UserInfoQuery;
import com.myblogs.entity.po.UserInfo;
import com.myblogs.entity.vo.PaginationResultVO;
import com.myblogs.entity.query.SimplePage;
import com.myblogs.mappers.UserInfoMapper;
import com.myblogs.service.UserInfoService;
import com.myblogs.utils.StringTools;


/**
 * 用户信息表 业务接口实现
 */
@Service("userInfoService")
public class UserInfoServiceImpl implements UserInfoService {

	@Resource
	private UserInfoMapper<UserInfo, UserInfoQuery> userInfoMapper;

	@Resource
	private JwtTools jwtTools;

	@Resource
	private AppConfig appConfig;

	@Resource
	private RedisComponent redisComponent;
	/**
	 * 根据条件查询列表
	 */
	@Override
	public List<UserInfo> findListByParam(UserInfoQuery param) {
		return this.userInfoMapper.selectList(param);
	}

	/**
	 * 根据条件查询列表
	 */
	@Override
	public Integer findCountByParam(UserInfoQuery param) {
		return this.userInfoMapper.selectCount(param);
	}

	/**
	 * 分页查询方法
	 */
	@Override
	public PaginationResultVO<UserInfo> findListByPage(UserInfoQuery param) {
		int count = this.findCountByParam(param);
		int pageSize = param.getPageSize() == null ? PageSize.SIZE15.getSize() : param.getPageSize();

		SimplePage page = new SimplePage(param.getPageNo(), count, pageSize);
		param.setSimplePage(page);
		List<UserInfo> list = this.findListByParam(param);
		PaginationResultVO<UserInfo> result = new PaginationResultVO(count, page.getPageSize(), page.getPageNo(), page.getPageTotal(), list);
		return result;
	}

	/**
	 * 新增
	 */
	@Override
	public Integer add(UserInfo bean) {
		return this.userInfoMapper.insert(bean);
	}

	/**
	 * 批量新增
	 */
	@Override
	public Integer addBatch(List<UserInfo> listBean) {
		if (listBean == null || listBean.isEmpty()) {
			return 0;
		}
		return this.userInfoMapper.insertBatch(listBean);
	}

	/**
	 * 批量新增或者修改
	 */
	@Override
	public Integer addOrUpdateBatch(List<UserInfo> listBean) {
		if (listBean == null || listBean.isEmpty()) {
			return 0;
		}
		return this.userInfoMapper.insertOrUpdateBatch(listBean);
	}

	/**
	 * 多条件更新
	 */
	@Override
	public Integer updateByParam(UserInfo bean, UserInfoQuery param) {
		StringTools.checkParam(param);
		return this.userInfoMapper.updateByParam(bean, param);
	}

	/**
	 * 多条件删除
	 */
	@Override
	public Integer deleteByParam(UserInfoQuery param) {
		StringTools.checkParam(param);
		return this.userInfoMapper.deleteByParam(param);
	}

	/**
	 * 根据Id获取对象
	 */
	@Override
	public UserInfo getUserInfoById(Long id) {
		return this.userInfoMapper.selectById(id);
	}

	@Override
	public UserInfo getUserInfoByEmail(String email) {
		return this.userInfoMapper.selectByEmail(email);
	}

	/**
	 * 根据Id修改
	 */
	@Override
	public Integer updateUserInfoById(UserInfo bean, Long id) {
		return this.userInfoMapper.updateById(bean, id);
	}

	/**
	 * 根据Id删除
	 */
	@Override
	public Integer deleteUserInfoById(Long id) {
		return this.userInfoMapper.deleteById(id);
	}

	@Override
	public UserInfoVO login(String email, String password) {
		UserInfo userInfo = this.userInfoMapper.selectByEmail(email);
		if (userInfo == null || !userInfo.getPassword().equals(password)) {
			throw new BusinessException("邮箱密码不存在或密码错误");
		}
		if ( ! userInfo.getStatus().equals(UserInfoStatusEnum.ACTIVE.getStatus())) {
			throw new BusinessException("账号不存在或被禁用");
		}
		UserInfo updateInfo = new UserInfo();
		updateInfo.setLastLoginTime(new Date());
		this.userInfoMapper.updateById(updateInfo, userInfo.getId());
		userInfo.setLastLoginTime(new Date());

		UserInfoVO userInfoVO = CopyTools.copy(userInfo, UserInfoVO.class);
		JwtUserInfoDto jwtUserInfoDto = CopyTools.copy(userInfo, JwtUserInfoDto.class);
		jwtUserInfoDto.setIssuedTime(System.currentTimeMillis());
		jwtUserInfoDto.setExpiredTime(appConfig.getJwtExpiration());
		String token = jwtTools.generateJwtToken(jwtUserInfoDto);

		userInfoVO.setJwtToken(token);
		userInfoVO.setIsAdmin(appConfig.getAdminEmail().equals(email));


		return userInfoVO;
	}


	@Override
	public void register(String email, String nickName, String password) {
		UserInfo userInfo = this.userInfoMapper.selectByEmail(email);
		if (userInfo != null) {
			throw new BusinessException("账号已存在");
		}
		Long userId = HuTools.getNewUserId();
		userInfo = new UserInfo();
		userInfo.setUserId(userId);
		userInfo.setEmail(email);
		userInfo.setNickname(nickName);
		userInfo.setPassword(StringTools.digestMd5(password));
		userInfo.setStatus(UserInfoStatusEnum.ACTIVE.getStatus());
		userInfo.setLastLoginTime(new Date());
		userInfo.setCreateTime(new Date());
		userInfo.setUpdateTime(new Date());
		userInfo.setDescription(Constants.DEFAULT_DESCRIPTION);
		userInfoMapper.insert(userInfo);

		return;
	}
}