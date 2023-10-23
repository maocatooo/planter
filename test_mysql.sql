CREATE TABLE product (
         product_id INT AUTO_INCREMENT PRIMARY KEY comment "商品ID",
         product_name VARCHAR(255) NOT NULL comment "商品名称",
         description TEXT comment "商品描述",
         price DECIMAL(10, 2) NOT NULL comment "商品价格",
         stock_quantity INT NOT NULL comment "库存数量",
         category VARCHAR(50) comment "商品类别 ",
         created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP comment "创建时间",
         updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP comment "修改时间"
);
alter table product
    comment '商品';

CREATE TABLE user (
      user_id INT AUTO_INCREMENT PRIMARY KEY COMMENT '用户ID',
      username VARCHAR(50) NOT NULL COMMENT '用户名称',
      email VARCHAR(100) UNIQUE NOT NULL COMMENT '电子邮件',
      password_hash CHAR(60) NOT NULL COMMENT '密码哈希',
      first_name VARCHAR(50) COMMENT '名字',
      last_name VARCHAR(50) COMMENT '姓氏',
      created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP COMMENT '创建时间',
      updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP COMMENT '更新时间'
);
alter table user
    comment '用户';

CREATE TABLE cart (
      cart_id INT AUTO_INCREMENT PRIMARY KEY COMMENT '购物车ID',
      user_id INT NOT NULL COMMENT '用户ID',
      product_id INT NOT NULL COMMENT '商品ID',
      quantity INT NOT NULL COMMENT '商品数量',
      added_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP COMMENT '添加时间',
      FOREIGN KEY (user_id) REFERENCES user(user_id),
      FOREIGN KEY (product_id) REFERENCES product(product_id)
);
alter table cart
    comment '购物车';


CREATE TABLE order_info (
        order_info_id INT AUTO_INCREMENT PRIMARY KEY COMMENT '订单ID',
        user_id INT NOT NULL COMMENT '用户ID',
        total_amount DECIMAL(10, 2) NOT NULL COMMENT '订单总金额',
        order_date TIMESTAMP DEFAULT CURRENT_TIMESTAMP COMMENT '订单日期',
        status ENUM('Pending', 'Processing', 'Shipped', 'Delivered') NOT NULL COMMENT '订单状态',
        shipping_address VARCHAR(255) NOT NULL COMMENT '配送地址',
        payment_method VARCHAR(50) NOT NULL COMMENT '付款方式',
        FOREIGN KEY (user_id) REFERENCES user(user_id)
);
alter table order_info
    comment '订单';

