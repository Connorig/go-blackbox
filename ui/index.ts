import {
    get,
    post,
    put,
    del,
    baseURL
} from '/@/utils/request';


// 物料周转率
export const analysisSkuTurnoverApi = (p) => get( baseURL+'/analysis/turnover/sku', p)

// 产品周转率
export const analysisSpuTurnoverApi = (p) => get( baseURL+'/analysis/turnover/spu', p)

// 呆滞物料分析
export const analysisSkuExcessApi = (p) => get( baseURL+'/analysis/excess/sku', p)
