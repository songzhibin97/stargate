import { FormErrors } from '../types';

// Email validation
export const validateEmail = (email: string): string | null => {
  if (!email) {
    return '邮箱不能为空';
  }
  
  const emailRegex = /^[^\s@]+@[^\s@]+\.[^\s@]+$/;
  if (!emailRegex.test(email)) {
    return '请输入有效的邮箱地址';
  }
  
  return null;
};

// Password validation
export const validatePassword = (password: string): string | null => {
  if (!password) {
    return '密码不能为空';
  }
  
  if (password.length < 8) {
    return '密码长度至少为8位';
  }
  
  // Check for at least one letter and one number
  const hasLetter = /[a-zA-Z]/.test(password);
  const hasNumber = /\d/.test(password);
  
  if (!hasLetter || !hasNumber) {
    return '密码必须包含至少一个字母和一个数字';
  }
  
  return null;
};

// Name validation
export const validateName = (name: string): string | null => {
  if (!name) {
    return '姓名不能为空';
  }
  
  if (name.length < 2) {
    return '姓名长度至少为2位';
  }
  
  if (name.length > 50) {
    return '姓名长度不能超过50位';
  }
  
  return null;
};

// Application name validation
export const validateApplicationName = (name: string): string | null => {
  if (!name) {
    return '应用名称不能为空';
  }
  
  if (name.length < 3) {
    return '应用名称长度至少为3位';
  }
  
  if (name.length > 100) {
    return '应用名称长度不能超过100位';
  }
  
  // Only allow letters, numbers, spaces, hyphens, and underscores
  const nameRegex = /^[a-zA-Z0-9\s\-_]+$/;
  if (!nameRegex.test(name)) {
    return '应用名称只能包含字母、数字、空格、连字符和下划线';
  }
  
  return null;
};

// Application description validation
export const validateApplicationDescription = (description: string): string | null => {
  if (!description) {
    return '应用描述不能为空';
  }
  
  if (description.length < 10) {
    return '应用描述长度至少为10位';
  }
  
  if (description.length > 500) {
    return '应用描述长度不能超过500位';
  }
  
  return null;
};

// Generic form validation
export const validateForm = (
  values: Record<string, any>,
  validators: Record<string, (value: any) => string | null>
): FormErrors => {
  const errors: FormErrors = {};
  
  Object.keys(validators).forEach(field => {
    const validator = validators[field];
    const value = values[field];
    const error = validator(value);
    
    if (error) {
      errors[field] = error;
    }
  });
  
  return errors;
};

// Check if form has errors
export const hasFormErrors = (errors: FormErrors): boolean => {
  return Object.keys(errors).length > 0;
};
