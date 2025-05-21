import {
    get,
    post,
    put,
    del,
    baseURL
} from '/@/utils/request';


export const analysisSkuTurnoverApi = (p) => get( baseURL+'/analysis/turnover/sku', p)

export const analysisSpuTurnoverApi = (p) => get( baseURL+'/analysis/turnover/spu', p)

export const analysisSkuExcessApi = (p) => get( baseURL+'/analysis/excess/sku', p)
