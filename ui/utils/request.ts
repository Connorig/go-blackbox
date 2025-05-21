import axios from 'axios';
import {ElMessage, ElMessageBox} from 'element-plus';
import {Session, Local} from '/@/utils/storage';

axios.defaults.headers.post['Content-Type'] = 'application/json'
export const baseURL = 'main'
// 配置新建一个 axios 实例
const service = axios.create({
    baseURL: import.meta.env.VITE_API_URL as any,
    timeout: 50000,
    // headers: { 'Content-Type': 'application/json' },
});

// 添加请求拦截器
service.interceptors.request.use(
    (config) => {
        // 在发送请求之前做些什么 token
        if (Session.get('token')) {
            config.headers.common['Authorization'] = "Bearer " + `${Session.get('token')}`;
            config.headers.common['token'] = `${Session.get('token')}`;
        }
        var themeConfig = Local.get("themeConfig")
        if (themeConfig && themeConfig.globalI18n) {
            config.headers.common['locale'] = themeConfig.globalI18n == "zh-cn" ? "zh_CN" : "en";
        } else {
            config.headers.common['locale'] = "zh_CN";
        }
        if (Local.get("userInfos")) {
            config.headers.common['orgId'] = Local.get('orgId') || "1"
            config.headers.common['deptId'] = Local.get('deptId') || "1"
            config.headers.common['operatorId'] = Local.get("userInfos").id || "1"
        } else {
            config.headers.common['orgId'] = "1"
            config.headers.common['deptId'] = "1"
            config.headers.common['operatorId'] = "1"
        }
        return config;
    },
    (error) => {
        // 对请求错误做些什么
        return Promise.reject(error);
    }
);

// 添加响应拦截器
service.interceptors.response.use(
    (response) => {
        // 对响应数据做点什么
        const res = response.data;
        if (res.code && res.code !== 200) {
            // `token` 过期或者账号已在别处登录
            // if (res.code === 401) {
            // 	Session.clear(); // 清除浏览器全部临时缓存
            // 	window.location.href = '/'; // 去登录页
            // 	ElMessageBox.alert('你已被登出，请重新登录', '提示', {})
            // 		.then(() => {})
            // 		.catch(() => {});
            // }
            // ElMessage.error(res.code + ": " + res.message)
            // return Promise.reject(service.interceptors.response);
            return response.data
        } else {
            // 导出报表的接口需要headers
            if (response.headers["content-disposition"] && response.headers["content-disposition"] !== null) {
                return response
            }
            // 返回数据
            return response.data;
        }
    },
    (error) => {
        // 对响应错误做点什么
        if (error.message.indexOf('timeout') != -1) {
            ElMessage.error('网络超时');
        } else if (error.message == 'Network Error') {
            ElMessage.error('网络连接错误');
        } else {
            if (error.response && error.response.data) ElMessage.error(error.response.statusText);
            else ElMessage.error('接口路径找不到');
        }
        return Promise.reject(error);
    }
);

export function get(url: string, params: object) {
    return service.request({
        url: url,
        method: 'get',
        params: params,
    });
};

export function post(url: string, params: object) {
    return service.request({
        url: url,
        method: 'post',
        data: params,
    });
};

export function put(url: string, params: object) {
    return service.request({
        url: url,
        method: 'put',
        data: params,
    });
};

export function del(url: string, params: object) {
    return service.request({
        url: url,
        method: 'delete',
        data: params,
    });
};

// export function exportXLS(url: string, params: object) {
//   return service.request({
//     method: 'GET',
//     url: url,
//     params: params,
//     headers: {
//       'Content-Type': 'application/json'
//       },
//     responseType: 'blob'
//   }).then(res => {
//     const link = document.createElement('a')
//     const blob = new Blob([res.data], { type: 'application/vnd.ms-excel' })
//     link.style.display = 'none'
//     link.href = URL.createObjectURL(blob)
//     let fileName = res.headers["content-disposition"]
//     fileName = decodeURI(escape(fileName.substring(fileName.indexOf("=")+1)))
//     link.setAttribute('download', fileName)
//     document.body.appendChild(link)
//     link.click()
//     document.body.removeChild(link)
//   })
// }

export function exportXLS(url: string, params: object) {
    return service.request({
        method: 'POST',
        url: url,
        data: params,
        headers: {
            'Content-Type': 'application/json'
        },
        responseType: 'blob'
    }).then(res => {
        const link = document.createElement('a')
        const blob = new Blob([res.data], {type: 'application/vnd.ms-excel'})
        link.style.display = 'none'
        link.href = URL.createObjectURL(blob)
        let fileName = res.headers["content-disposition"]
        fileName = decodeURI(escape(fileName.substring(fileName.indexOf("=") + 1)))
        link.setAttribute('download', fileName)
        document.body.appendChild(link)
        link.click()
        document.body.removeChild(link)
    })
}

// 导出 axios 实例
export default service;
