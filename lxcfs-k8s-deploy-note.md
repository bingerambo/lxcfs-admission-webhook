


本文主要介绍了自己对lxcfs的k8s方式部署的总结


## lxcfs
lxcfs 是一个开源的 FUSE（用户态文件系统）实现来支持 LXC 容器，它也可以支持 Docker 容器。让容器内的应用在读取内存和 CPU 信息的时候通过 lxcfs 的映射，转到自己的通过对 cgroup 中容器相关定义信息读取的虚拟数据上。




runc没有做到完全隔离/proc、/sys路径下的文件，所以容器内通过top、free等命令看到的数据都是物理机上的。对于习惯了虚机，物理机的同学来说不太友好，而且这些命令似乎也失去了本质意义。lxcfs作用就是将容器内/proc、/sys文件与物理机隔离，让top等命令显示容器内真实数据。


说明：

lxcfs是以用户空间文件系统（Filesystem in Userspace）为基础，以cgroup技术实现的用户空间的虚拟文件系统。

容器中读取lxcfs文件系统

lxcfs程序启动时会指定一个路径（如下图是/var/lib/lxcfs）作为挂载点，以后读取这个路径的下文件（cgroup、proc、sys）vfs都会调用内核fuse，fuse回调lxcfs实现的文件操作函数。容器内读取lxcfs文件系统中的数据时，通过gblic系统调用vfs接口然后转到fuse内核模块，内核模块fuse回调lxcfs程序中实现的回调函数，获取容器的cgroup，然后去宿主机对应cgroup下读取并计算后得到容器的实际mem、cpu等信息。lxcfs将物理机的cgroups挂载到运行时环境/run/lxcfs/controllers，但直接在物理机上看不见，因为程序中用unshare做了mounts namespace隔离。lxcfs程序中所有的cgroups信息都从/run/lxcfs/controllers下获得。



关于lxcfs的工作原理已有很多文章介绍，这里不再赘述。


lxcfs的k8s方式部署，主要有2个步骤内容：
1. 运行lxcfs-daemonset，保证每个节点都有运行lxcfs服务
2. 运行lxcfs-admission-webhook，保证在某namespace下的业务pod能够自动注入lxcfs挂载目录，无需业务pod手动配置lxcfs挂载



## AdmissionWebhook

```
什么是AdmissionWebhook

什么是AdmissionWebhook，就要先了解K8S中的admission controller, 按照官方的解释是： admission controller是拦截(经过身份验证)API Server请求的网关，并且可以修改请求对象或拒绝请求。

简而言之，它可以认为是拦截器，类似web框架中的middleware。

K8S默认提供很多内置的admission controller，通过kube-apiserver启动命令参数可以 查看到支持的admission controller plugin有哪些。

kube-apiserver --help |grep enable-admission-plugins


MutatingAdmissionWebhook, ValidatingAdmissionWebhook

MutatingAdmissionWebhook: 做修改操作的AdmissionWebhook 
ValidatingAdmissionWebhook: 做验证操作的AdmissionWebhook

```

## lxcfs-admission-webhook

在kubernetes中安装lxcfs
这里阿里云提供了kubernetes的解决方案，能够通过yaml自动化安装和启用lxcfs
https://github.com/denverdino/lxcfs-admission-webhook

**说明，这个lxcfs-admission-webhook项目的k8s方式部署会有问题，项目的issue中也有讨论**

1. lxcfs-daemonset重启会报错
2. 部署lxcfs，启动业务pod后，再重启lxcfs，发现业务pod内无法查看cpu和内存信息，如：无法正确执行free命令

所以，该项目无法直接进行生产环境部署，需要进行优化

## 优化部署方式

具体配置可以参考我的lxcfs-admission-webhook项目：



```

# Kubernetes 1.9.0 or above with the admissionregistration.k8s.io/v1beta1 API enabled. Verify that by the following command:

# kubectl api-versions | grep admissionregistration.k8s.io/v1beta1
# The result should be:

# admissionregistration.k8s.io/v1beta1
# In addition, the MutatingAdmissionWebhook and ValidatingAdmissionWebhook admission controllers should be added and listed in the correct order in the admission-control flag of kube-apiserver.


# MutatingAdmissionWebhook ValidatingAdmissionWebhook


```

### 部署

提前准备好lxcfs镜像和lxcfs-admission-webhook镜像 
```
## lxcfs-admission-webhook 在k8s1.20环境中的部署，检查api接口

kubectl api-versions | grep admissionregistration.k8s.io/v1beta1



### Deploy
#### Deploy lxcfs to worker nodes

kubectl apply -f deployment/lxcfs-daemonset.yaml

#### Install injector with lxcfs-admission-webhook
deployment/install.sh


### install.sh  installed webhook

### kubectl delete -f deployment/deployment.yaml
### kubectl apply -f deployment/deployment.yaml




```



## 测试

```

Test
Enable the namespace for injection


kubectl label namespace default lxcfs-admission-webhook=enabled
Note: All the new created pod under the namespace will be injected with LXCFS
```

```shell


Enable the namespace for injection


# kubectl label namespace default lxcfs-admission-webhook=enabled
# Note: All the new created pod under the namespace will be injected with LXCFS


### 修改测试namespace下的webhook
#### 删除原先设置的 ns:default lxcfs-admission-webhook
# kubectl label namespace default lxcfs-admission-webhook-

## 自定义namespace 设置 lxcfs-admission-webhook
kubectl create namespace test-lxcfs
kubectl label namespace test-lxcfs lxcfs-admission-webhook=enabled


#### 在业务pod的配置文件 web.yaml中 设置namespace为test-lxcfs




### Deploy the test deployment
kubectl apply -f deployment/web.yaml

kubectl delete -f deployment/web.yaml
kubectl apply -f deployment/web.yaml


kubectl delete -f deployment/web111.yaml
kubectl apply -f deployment/web111.yaml


### namespace test-lxcfs web pod
kubectl delete -f deployment/web2.yaml
kubectl apply -f deployment/web2.yaml

### Inspect the resource inside container

kubectl get pod

```



## lxcfs对容器资源隔离

容器资源显示效果

```shell



# cpu数量 最小个数为1

#
# cat /proc/cpuinfo  | grep processor
processor       : 0
#

# 内存大小

# free -hm
             total       used       free     shared    buffers     cached
Mem:          256M       3.0M       252M         0B         0B       344K
-/+ buffers/cache:       2.7M       253M
Swap:           0B         0B         0B
#


# 共享内存

# ipcs -m

------ Shared Memory Segments --------
key        shmid      owner      perms      bytes      nattch     status

#
#






```





## 常用操作命令

命令杂记，不能按顺序执行
```
# 制作自制镜像

cd /home/wangb/lxcfs-test/custom-lxcfs-image
docker build -t lxcfs:dev2 .



docker images |grep lxcfs


docker run -dit --name lxcfsdev lxcfs:dev /bin/sh

cidname=lxcfsdev
file_source=container_remount_lxcfs.sh
target_path=/container_remount_lxcfs.sh
docker cp ${file_source} ${cidname}:${target_path}



docker exec -it lxcfsdev /bin/sh

cidname=lxcfsdev
midname=lxcfs:dev
docker commit -m 'make image' ${cidname} ${midname}





# fusermount卸载

nsenter -m/proc/1/ns/mnt /home/wangb/lxcfs-test/fusermount -u /var/lib/lxc/lxcfs

```





## 自定义配置

具体内容可以参考我的lxcfs-admission-webhook项目：
http://bingerambo.com

### lxcfs start.sh
lxcfs镜像的启动配置

```shell

#!/bin/bash

# Cleanup
nsenter -m/proc/1/ns/mnt fusermount -u /var/lib/lxc/lxcfs 2> /dev/null || true
nsenter -m/proc/1/ns/mnt [ -L /etc/mtab ] || \
        sed -i "/^lxcfs \/var\/lib\/lxc\/lxcfs fuse.lxcfs/d" /etc/mtab

# Prepare
mkdir -p /usr/local/lib/lxcfs /var/lib/lxc/lxcfs

# Update lxcfs
cp -f /lxcfs/lxcfs /usr/local/bin/lxcfs
cp -f /lxcfs/liblxcfs.so /usr/local/lib/lxcfs/liblxcfs.so


# Mount
exec nsenter -m/proc/1/ns/mnt /usr/local/bin/lxcfs /var/lib/lxc/lxcfs/

```

### lxcfs Dockfie


```shell

FROM centos:7 as build
RUN yum -y update
RUN yum -y install fuse-devel pam-devel wget install gcc automake autoconf libtool make
ENV LXCFS_VERSION 3.1.2
RUN wget https://linuxcontainers.org/downloads/lxcfs/lxcfs-$LXCFS_VERSION.tar.gz && \
    mkdir /lxcfs && tar xzvf lxcfs-$LXCFS_VERSION.tar.gz -C /lxcfs  --strip-components=1 && \
    cd /lxcfs && ./configure && make

FROM centos:7
RUN yum -y install fuse fuse-lib fuse-devel
STOPSIGNAL SIGINT
COPY --from=build /lxcfs/lxcfs /usr/local/bin/lxcfs
COPY --from=build /lxcfs/.libs/liblxcfs.so /usr/local/lib/lxcfs/liblxcfs.so
COPY --from=build /lxcfs/lxcfs /lxcfs/lxcfs
COPY --from=build /lxcfs/.libs/liblxcfs.so /lxcfs/liblxcfs.so
COPY --from=build /usr/lib64/libfuse.so.2.9.2 /usr/lib64/libfuse.so.2.9.2
COPY --from=build /usr/lib64/libulockmgr.so.1.0.1 /usr/lib64/libulockmgr.so.1.0.1
RUN ln -s /usr/lib64/libfuse.so.2.9.2 /usr/lib64/libfuse.so.2 && \
    ln -s /usr/lib64/libulockmgr.so.1.0.1 /usr/lib64/libulockmgr.so.1
COPY start.sh /
RUN chmod 750 /start.sh
CMD ["/start.sh"]


```

### lxcfs DaemonSet
这里我新增了lifecycle的前后处理
```yaml

apiVersion: apps/v1
kind: DaemonSet
metadata:
  name: lxcfs
  namespace: kube-system
  labels:
    app: lxcfs
spec:
  selector:
    matchLabels:
      app: lxcfs
  template:
    metadata:
      labels:
        app: lxcfs
    spec:
      hostPID: true
      tolerations:
      - key: node-role.kubernetes.io/master
        effect: NoSchedule
      containers:
      - name: lxcfs
        # image: registry.cn-hangzhou.aliyuncs.com/denverdino/lxcfs:3.1.2
        image: lxcfs:dev
        # imagePullPolicy: Always
        imagePullPolicy: IfNotPresent
        # command: [ "/bin/sh","-c","chmod u+x /start.sh;/start.sh" ]
        command: [ "/bin/sh","-c","/start.sh" ]
        securityContext:
          privileged: true
        # custom config      
        lifecycle:
          postStart:
            exec:
              command: ["/bin/sh", "-c", "chmod u+x /tmp/container_remount_lxcfs.sh;/tmp/container_remount_lxcfs.sh  2> /dev/null  || true"]
          preStop:
            exec:
              # command: ["/bin/sh", "-c", "nsenter -m/proc/1/ns/mnt fusermount -u /var/lib/lxc/lxcfs 2> /dev/null || true"]
              command: ["/bin/sh", "-c", "nsenter -m/proc/1/ns/mnt fusermount -u /var/lib/lxc/lxcfs 2> /dev/null  || true"]
        volumeMounts:
        - name: cgroup
          mountPath: /sys/fs/cgroup
        - name: lxcfs
          # mountPath: /var/lib/lxcfs
          mountPath: /var/lib/lxc/lxcfs
          mountPropagation: Bidirectional
        - name: usr-local
          mountPath: /usr/local
        # custom config
        - name: docker-bin
          mountPath: /usr/bin/docker
        - name: docker-sock
          mountPath: /var/run/docker.sock
        - name: libltdl-so
          mountPath: /usr/lib64/libltdl.so.7
        # configmap
        - name: lxcfs-remount
          mountPath: /tmp/container_remount_lxcfs.sh
      volumes:
      - name: cgroup
        hostPath:
          path: /sys/fs/cgroup
      - name: usr-local
        hostPath:
          path: /usr/local
      - name: lxcfs
        hostPath:
          # path: /var/lib/lxcfs
          path: /var/lib/lxc/lxcfs
          type: DirectoryOrCreate
      # - name: lxcfs
      #   emptyDir: {}
      # custom config
      - name: docker-bin
        hostPath:
          path: /usr/bin/docker
      - name: docker-sock
        hostPath:
          path: /var/run/docker.sock
      - name: libltdl-so
        hostPath:
          path: /usr/lib64/libltdl.so.7
      - name: lxcfs-remount
        hostPath:
          path: /tmp/container_remount_lxcfs.sh



```



### container_remount_lxcfs.sh
解决lxcfs重启导致业务pod查看资源异常的重新挂载脚本，这里我新增了 检测lxcfs状态的步骤
```shell

#! /bin/bash

## 该脚本适用于lxcfs容器化部署

PATH=$PATH:/bin
LXCFS="/var/lib/lxc/lxcfs"
LXCFS_ROOT_PATH="/var/lib/lxc"


## 1. check lxcfs process state

# cpuinfo
# diskstats
# loadavg
# meminfo
# stat
# swaps
# uptime

proc_num=$(ls ${LXCFS}/proc/ |wc -l)

# while [[ ${proc_num} != 7 ]]; do
while [[ ${proc_num} == 0 ]]; do
    #statements
    echo "check lxcfs process state failed: not find all ${LXCFS}/proc..."
    echo $(ls ${LXCFS}/proc/)
    sleep 2s
    proc_num=$(ls ${LXCFS}/proc/ |wc -l)

done

## 2. find containers to remount
# PATH=$PATH:/bin
# LXCFS="/var/lib/lxc/lxcfs"
# LXCFS_ROOT_PATH="/var/lib/lxc"

containers=$(docker ps | grep -v pause  | grep -v calico | awk '{print $1}' | grep -v CONTAINE)

#-v /var/lib/lxc/lxcfs/proc/cpuinfo:/proc/cpuinfo:rw
#-v /var/lib/lxc/lxcfs/proc/diskstats:/proc/diskstats:rw
#-v /var/lib/lxc/lxcfs/proc/meminfo:/proc/meminfo:rw
#-v /var/lib/lxc/lxcfs/proc/stat:/proc/stat:rw
#-v /var/lib/lxc/lxcfs/proc/swaps:/proc/swaps:rw
#-v /var/lib/lxc/lxcfs/proc/uptime:/proc/uptime:rw
#-v /var/lib/lxc/lxcfs/proc/loadavg:/proc/loadavg:rw
#-v /var/lib/lxc/lxcfs/sys/devices/system/cpu/online:/sys/devices/system/cpu/online:rw
for container in $containers;do
    mountpoint=$(docker inspect --format '{{ range .Mounts }}{{ if eq .Destination "/var/lib/lxc" }}{{ .Source }}{{ end }}{{ end }}' $container)
    if [ "$mountpoint" = "$LXCFS_ROOT_PATH" ];then
        echo "remount $container"
        PID=$(docker inspect --format '{{.State.Pid}}' $container)

        # mount /proc
        for file in meminfo cpuinfo loadavg stat diskstats swaps uptime;do
            echo nsenter --target $PID --mount --  mount -B "$LXCFS/proc/$file" "/proc/$file"
            nsenter --target $PID --mount --  mount -B "$LXCFS/proc/$file" "/proc/$file"
        done
        # mount /sys
        for file in online;do
            echo nsenter --target $PID --mount --  mount -B "$LXCFS/sys/devices/system/cpu/$file" "/sys/devices/system/cpu/$file"
            nsenter --target $PID --mount --  mount -B "$LXCFS/sys/devices/system/cpu/$file" "/sys/devices/system/cpu/$file"
        done 
    fi 
done


# 1. 将宿主机上lxcfs的上一层目录挂到容器里
# -v /var/lib/lxc/:/var/lib/lxc/:shared  \
# -v /var/lib/lxc/lxcfs/proc/uptime:/proc/uptime \
# -v /var/lib/lxc/lxcfs/proc/swaps:/proc/swaps  \
# -v /var/lib/lxc/lxcfs/proc/stat:/proc/stat  \
# -v /var/lib/lxc/lxcfs/proc/diskstats:/proc/diskstats \
# -v /var/lib/lxc/lxcfs/proc/meminfo:/proc/meminfo \
# -v /var/lib/lxc/lxcfs/proc/cpuinfo:/proc/cpuinfo
# 2. 在systemd的增加ExecStartPost=remount_lxcfs
# 3. 在lxcfs进程重启之后，在容器里再mount一次mount -B "/var/lib/lxc/lxcfs/proc/$file" "/proc/$file"

```



### lxcfs-admission-webhook build
原来项目的lxcfs-admission-webhook 无法满足我的项目需要，
我这里修改了lxcfs-admission-webhook代码，重新进行了构建

```shell

#!/bin/bash

DOCKER_USER=bingerambo

#dep ensure -v

# go mod
CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o lxcfs-admission-webhook
docker build --no-cache -t lxcfs-admission-webhook:dev .
#docker build --no-cache -t registry.cn-hangzhou.aliyuncs.com/${DOCKER_USER}/lxcfs-admission-webhook:v1 .
#rm -rf lxcfs-admission-webhook
#
#docker push registry.cn-hangzhou.aliyuncs.com/${DOCKER_USER}/lxcfs-admission-webhook:v1

```



### web deployment
业务pod测试部署
```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: web
  namespace: test-lxcfs
spec:
  replicas: 2
  selector:
    matchLabels:
      app: web
  template:
    metadata:
      labels:
        app: web
    spec:
      containers:
      - name: web
        image: httpd:2.4.32
        imagePullPolicy: Always
        resources:
          requests:
            memory: "256Mi"
            cpu: "500m"
          limits:
            memory: "256Mi"
            cpu: "500m"
        

```




## 报错问题


https://github.com/lxc/lxcfs


### 问题1 部署lxcfs daemonset报错
``` shell
[root@node1 lxcfs-test]# kubectl logs lxcfs-sffmt
mount namespace: 5
hierarchies:
  0: fd:   6: memory
  1: fd:   7: hugetlb
  2: fd:   8: cpuset
  3: fd:   9: perf_event
  4: fd:  10: cpuacct,cpu
  5: fd:  11: devices
  6: fd:  12: pids
  7: fd:  13: freezer
  8: fd:  14: blkio
  9: fd:  15: net_prio,net_cls
 10: fd:  16: name=systemd
fuse: mountpoint is not empty
fuse: if you are sure this is safe, use the 'nonempty' mount option

```

https://github.com/denverdino/lxcfs-admission-webhook/issues/8

对于fuser 所提示“noempty”挂载选项应该如何去配置在daemonset 当中呢？
是可能因为在挂载lxcfs的时候出现挂载的目录当中已经存在数据造成的。


问题解决：
说明 hostpath：/var/lib/lxc/lxcfs 存在数据，如下
/var/lib/lxc/lxcfs/proc/



rm -rf /var/lib/lxcfs



### 问题2  lxcfs pod重启会失败
```shell
lxcfs-5pxkb                                          0/1     RunContainerError 


Events:
  Type     Reason     Age                From               Message
  ----     ------     ----               ----               -------
  Normal   Scheduled  60s                default-scheduler  Successfully assigned default/lxcfs-5pxkb to node2
  Warning  Failed     59s (x2 over 59s)  kubelet            Error: failed to start container "lxcfs": Error response from daemon: OCI runtime create failed: container_linux.go:380: starting container process caused: process_linux.go:545: container init caused: rootfs_linux.go:76: mounting "/var/lib/lxcfs/proc/loadavg" to rootfs at "/proc/loadavg" caused: mount through procfd: not a directory: unknown: Are you trying to mount a directory onto a file (or vice-versa)? Check if the specified host path exists and is the expected type
  Warning  Failed     45s                kubelet            Error: failed to start container "lxcfs": Error response from daemon: OCI runtime create failed: container_linux.go:380: starting container process caused: process_linux.go:545: container init caused: rootfs_linux.go:76: mounting "/var/lib/lxcfs/proc/cpuinfo" to rootfs at "/proc/cpuinfo" caused: mount through procfd: not a directory: unknown: Are you trying to mount a directory onto a file (or vice-versa)? Check if the specified host path exists and is the expected type
  Normal   Pulled     21s (x4 over 60s)  kubelet            Container image "registry.cn-hangzhou.aliyuncs.com/denverdino/lxcfs:3.1.2" already present on machine
  Normal   Created    21s (x4 over 60s)  kubelet            Created container lxcfs
  Warning  Failed     21s                kubelet            Error: failed to start container "lxcfs": Error response from daemon: OCI runtime create failed: container_linux.go:380: starting container process caused: process_linux.go:545: container init caused: rootfs_linux.go:76: mounting "/var/lib/lxcfs/proc/meminfo" to rootfs at "/proc/meminfo" caused: mount through procfd: not a directory: unknown: Are you trying to mount a directory onto a file (or vice-versa)? Check if the specified host path exists and is the expected type
  Warning  BackOff    6s (x4 over 58s)   kubelet            Back-off restarting failed container




```




lxcfs mount through procfd: not a directory: unknown: Are you trying to mount a directory onto a file (or vice-versa)? Check if the specified host path exists and is the expected type


因为 lxcfs-daemonset pod的namespace 和 lxcfs-webhook作用的namespace相同，如：default，  且没有删除掉lxcfs-webhook，导致启动 lxcfs-daemonset的挂载路径报错

修改下lxcfs-daemonset 的namespace 为 kube-system，与lxcfs-webhook作用的namespace区别开来，重新部署lxcfs-daemonset正常






### 问题3 run "mount proc /proc -t proc"


```shell
[root@node1 lxcfs-test]# kubectl exec -it web-fdf588795-p9f52 sh
kubectl exec [POD] [COMMAND] is DEPRECATED and will be removed in a future version. Use kubectl exec [POD] -- [COMMAND] instead.
#
#
# free -m
             total       used       free     shared    buffers     cached
Mem:           256          8        247          0          0          0
-/+ buffers/cache:          8        247
Swap:            0          0          0
#
#
# free -m
Error: /proc must be mounted
  To mount /proc at boot you need an /etc/fstab line like:
      proc   /proc   proc    defaults
  In the meantime, run "mount proc /proc -t proc"
#
```


lxcfs-daemonset pod 删除时，挂载目录/var/lib/lxcfs 也会删除
待 lxcfs-daemonset 重启时，业务容器pod的free查询挂载点报错

当lxcfs服务重启后，需要对容器挂载的响应目录进行重新挂载remount







### 问题4 Transport endpoint is not connected
重启lxcfs daemonset，业务pod无法查看容器cpu和内存信息
```shell
# cat cat /sys/devices/system/cpu/online
cat: cat: No such file or directory
cat: /sys/devices/system/cpu/online: Transport endpoint is not connected
#
#
#
#
# cat /proc/cpuinfo
cat: /proc/cpuinfo: Transport endpoint is not connected
#
#
#
# ls /proc
1    acpi       cpuinfo    execdomains  ioports    keys        mdstat   net           self      sysrq-trigger  version
10   buddyinfo  crypto     fb           ipmi       kmsg        meminfo  pagetypeinfo  slabinfo  sysvipc        vmallocinfo
114  bus        devices    filesystems  irq        kpagecount  misc     partitions    softirqs  timer_list     vmstat
125  cgroups    diskstats  fs           kallsyms   kpageflags  modules  sched_debug   stat      timer_stats    zoneinfo
8    cmdline    dma        interrupts   kcore      loadavg     mounts   schedstat     swaps     tty
9    consoles   driver     iomem        key-users  locks       mtrr     scsi          sys       uptime
# ls /pro       ^C
#
#
#
# cat /proc/cpuinfo
cat: /proc/cpuinfo: Transport endpoint is not connected
# cat /proc/version
Linux version 3.10.0-1127.el7.x86_64 (mockbuild@kbuilder.bsys.centos.org) (gcc version 4.8.5 20150623 (Red Hat 4.8.5-39) (GCC) ) #1 SMP Tue Mar 31 23:36:51 UTC 2020
#
```


问题分析lxcfs daemonset的挂载点问题，对已进行lxcfs挂载的容器，重新执行挂载
```shell
[root@node2 lxcfs-test]# sh container_remount_lxcfs.sh
remount aec74195b6e1
nsenter --target 5579 --mount -- mount -B /var/lib/lxc/lxcfs/proc/meminfo /proc/meminfo
nsenter --target 5579 --mount -- mount -B /var/lib/lxc/lxcfs/proc/cpuinfo /proc/cpuinfo
nsenter --target 5579 --mount -- mount -B /var/lib/lxc/lxcfs/proc/loadavg /proc/loadavg
nsenter --target 5579 --mount -- mount -B /var/lib/lxc/lxcfs/proc/stat /proc/stat
nsenter --target 5579 --mount -- mount -B /var/lib/lxc/lxcfs/proc/diskstats /proc/diskstats
nsenter --target 5579 --mount -- mount -B /var/lib/lxc/lxcfs/proc/swaps /proc/swaps
nsenter --target 5579 --mount -- mount -B /var/lib/lxc/lxcfs/proc/uptime /proc/uptime
nsenter --target 5579 --mount -- mount -B /var/lib/lxc/lxcfs/sys/devices/system/cpu/online /sys/devices/system/cpu/online
remount 344fdf47b13c
nsenter --target 5107 --mount -- mount -B /var/lib/lxc/lxcfs/proc/meminfo /proc/meminfo
nsenter --target 5107 --mount -- mount -B /var/lib/lxc/lxcfs/proc/cpuinfo /proc/cpuinfo
nsenter --target 5107 --mount -- mount -B /var/lib/lxc/lxcfs/proc/loadavg /proc/loadavg
nsenter --target 5107 --mount -- mount -B /var/lib/lxc/lxcfs/proc/stat /proc/stat
nsenter --target 5107 --mount -- mount -B /var/lib/lxc/lxcfs/proc/diskstats /proc/diskstats
nsenter --target 5107 --mount -- mount -B /var/lib/lxc/lxcfs/proc/swaps /proc/swaps
nsenter --target 5107 --mount -- mount -B /var/lib/lxc/lxcfs/proc/uptime /proc/uptime
nsenter --target 5107 --mount -- mount -B /var/lib/lxc/lxcfs/sys/devices/system/cpu/online /sys/devices/system/cpu/online
[root@node2 lxcfs-test]#
[root@node2 lxcfs-test]#
[root@node2 lxcfs-test]#

```


解决方式：重新构建lxcfs镜像，修改lxcfs-admission-webhook，再进行编译构建

关于remount_lxcfs的这个功能

https://github.com/pouchcontainer/blog/issues/3






### 问题5




  Warning  Failed     21s (x3 over 36s)  kubelet            Error: failed to start container "lxcfs": Error response from daemon: error while creating mount source path '/var/lib/lxc/lxcfs': mkdir /var/lib/lxc/lxcfs: file exists






```

 Normal   Started              6s (x3 over 24s)  kubelet            Started container lxcfs
  Warning  FailedPostStartHook  6s (x3 over 24s)  kubelet            Exec lifecycle hook ([/bin/sh -c /container_remount_lxcfs.sh || true]) for Container "lxcfs" in Pod "lxcfs-d9xf4_kube-system(019fba71-758a-411f-a538-1feb64ff5403)" failed - error: command '/bin/sh -c /container_remount_lxcfs.sh || true' exited with 126: , message: "OCI runtime exec failed: exec failed: cannot exec a container that has stopped: unknown\r\n"
  Normal   Killing              6s (x3 over 24s)  kubelet            FailedPostStartHook
  Warning  FailedPreStopHook    6s (x3 over 24s)  kubelet            Exec lifecycle hook ([/bin/sh -c nsenter -m/proc/1/ns/mnt fusermount -u /var/lib/lxc/lxcfs 2> /dev/null || true]) for Container "lxcfs" in Pod "lxcfs-d9xf4_kube-system(019fba71-758a-411f-a538-1feb64ff5403)" failed - error: rpc error: code = Unknown desc = container not running (lxcfs), message: ""
  Warning  BackOff              6s (x3 over 22s)  kubelet            Back-off restarting failed container


```




问题分析

This is because the image you are using to start the container is not supported for underlying hardware architecture.


容器不支持底层操作？？？由于脚本和自定义添加的命令处理，使用了底层程序但是容器中没有，如：docker命令，需要在容器中添加设置








### 问题6

重要说明，挂载路径被删除后导致的异常，则需要fusermount处理下。要保证业务pod容器中的挂载根路径（/var/lib/lxc）不能删除。 


- /var/lib/lxc 目录不能删除

- /var/lib/lxc/lxcfs 可以删除

- lxcfs和lxcfs-webhook的namespace不能相同，否则当lxcfs重启时，会由于lxcfs-webhook的回调处理导致lxcfs启动异常

## 参考

参考文章

https://github.com/lxc/lxcfs

https://www.helpsystems.com/resources/guides/unix-load-average-part-1-how-it-works

https://github.com/libfuse/libfuse

https://github.com/lxc/lxcfs

https://www.helpsystems.com/resources/guides/unix-load-average-part-1-how-it-works


玩转K8S AdmissionWebhook
https://cloud.tencent.com/developer/article/1445760

Kubernetes中Sidecar生命周期管理
https://blog.csdn.net/u012986012/article/details/108709892


为容器的生命周期事件设置处理函数
https://kubernetes.io/zh/docs/tasks/configure-pod-container/attach-handler-lifecycle-event/