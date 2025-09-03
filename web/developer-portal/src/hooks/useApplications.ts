import { useQuery, useMutation, useQueryClient } from 'react-query';
import { message } from 'antd';
import { 
  Application, 
  CreateApplicationRequest, 
  UpdateApplicationRequest, 
  ApplicationListResponse,
  PaginationParams 
} from '../types';
import { apiService } from '../services/api';

export const useApplications = (params?: PaginationParams) => {
  return useQuery<ApplicationListResponse, Error>(
    ['applications', params],
    () => apiService.getApplications(params),
    {
      staleTime: 30000, // 30 seconds
      refetchOnWindowFocus: false,
      onError: (error) => {
        message.error(`获取应用列表失败: ${error.message}`);
      },
    }
  );
};

export const useApplication = (id: string) => {
  return useQuery<Application, Error>(
    ['application', id],
    () => apiService.getApplication(id),
    {
      enabled: !!id,
      staleTime: 30000,
      onError: (error) => {
        message.error(`获取应用详情失败: ${error.message}`);
      },
    }
  );
};

export const useCreateApplication = () => {
  const queryClient = useQueryClient();
  
  return useMutation<Application, Error, CreateApplicationRequest>(
    (data) => apiService.createApplication(data),
    {
      onSuccess: () => {
        message.success('应用创建成功');
        queryClient.invalidateQueries(['applications']);
      },
      onError: (error) => {
        message.error(`创建应用失败: ${error.message}`);
      },
    }
  );
};

export const useUpdateApplication = () => {
  const queryClient = useQueryClient();
  
  return useMutation<Application, Error, { id: string; data: UpdateApplicationRequest }>(
    ({ id, data }) => apiService.updateApplication(id, data),
    {
      onSuccess: (data) => {
        message.success('应用更新成功');
        queryClient.invalidateQueries(['applications']);
        queryClient.invalidateQueries(['application', data.id]);
      },
      onError: (error) => {
        message.error(`更新应用失败: ${error.message}`);
      },
    }
  );
};

export const useDeleteApplication = () => {
  const queryClient = useQueryClient();
  
  return useMutation<void, Error, string>(
    (id) => apiService.deleteApplication(id),
    {
      onSuccess: () => {
        message.success('应用删除成功');
        queryClient.invalidateQueries(['applications']);
      },
      onError: (error) => {
        message.error(`删除应用失败: ${error.message}`);
      },
    }
  );
};

export const useRegenerateAPIKey = () => {
  const queryClient = useQueryClient();
  
  return useMutation<{ api_key: string; message: string }, Error, string>(
    (id) => apiService.regenerateAPIKey(id),
    {
      onSuccess: (_, id) => {
        message.success('API Key 重新生成成功');
        queryClient.invalidateQueries(['applications']);
        queryClient.invalidateQueries(['application', id]);
      },
      onError: (error) => {
        message.error(`重新生成 API Key 失败: ${error.message}`);
      },
    }
  );
};
